package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	goversion "go/version"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func compareGoVersion(left, right string) int { return goversion.Compare("go"+left, "go"+right) }

func writeProvisionOverlay(temp string, opts *options, replacements map[string]string) error {
	data, err := json.Marshal(Overlay{Replace: replacements})
	if err != nil {
		return err
	}
	path := filepath.Join(temp, "provision-overlay.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	flags := opts.buildFlags[:0]
	for _, flag := range opts.buildFlags {
		if !strings.HasPrefix(flag, "-overlay=") {
			flags = append(flags, flag)
		}
	}
	opts.buildFlags = append(flags, "-overlay="+path)
	return nil
}

// moduleRoot follows Go's module discovery. A temporary workspace uses the
// original root, so imports, -C, relative replacements and package flags retain
// their meaning.
func moduleRoot(dir string) string {
	for {
		if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// moduleWorkspaceInput represents one module without changing its language or
// GODEBUG defaults. The caller may overlay a selected -modfile first.
func moduleWorkspaceInput(ctx context.Context, dir, root, temp string, replacements map[string]string) (string, error) {
	source := filepath.Join(temp, "module-input.work")
	module, err := readModuleFile(filepath.Join(root, "go.mod"), replacements)
	if err != nil {
		return "", err
	}
	modfile := filepath.Join(temp, "module-settings.mod")
	if err := os.WriteFile(modfile, module, 0600); err != nil {
		return "", err
	}
	out, err := goTool(ctx, dir, nil, "mod", "edit", "-print", "-modfile="+modfile)
	if err != nil {
		return "", err
	}
	settingsGo := moduleDirective([]byte(out), "go")
	if settingsGo == "" {
		settingsGo = defaultGoModVersion
	}
	settingsDebug, err := formattedGoDebug(out)
	if err != nil {
		return "", err
	}
	// Go compares workspace module roots with its native working-directory
	// spelling. Absolute slash-normalized paths fail that comparison on Windows.
	workspaceGo := settingsGo
	if compareGoVersion(workspaceGo, "1.25.0") < 0 {
		workspaceGo = "1.25.0"
	}
	data := []byte("go " + workspaceGo + "\nuse " + strconv.Quote(root) + "\n")
	hasDefault := false
	for _, setting := range settingsDebug {
		hasDefault = hasDefault || setting.Key == "default"
		data = append(data, []byte("godebug "+setting.Key+"="+setting.Value+"\n")...)
	}
	if value := goDebugDefault(settingsGo); !hasDefault && workspaceGo != settingsGo && value != "" {
		data = append(data, []byte("godebug default="+value+"\n")...)
	}
	if err := os.WriteFile(source, data, 0600); err != nil {
		return "", err
	}
	return source, nil
}

func provideMiniVendorWorkspace(ctx context.Context, dir, root, temp string, replacements map[string]string) (string, *os.File, error) {
	source, err := moduleWorkspaceInput(ctx, dir, root, temp, replacements)
	if err != nil {
		return "", nil, err
	}
	work, err := provideMiniWorkspace(ctx, dir, source, temp, replacements)
	if err != nil {
		return "", nil, err
	}
	return vendorWorkspace(ctx, work, vendorManifest{dir: filepath.Join(root, "vendor"), base: root, module: true}, replacements)
}

// vendorManifest is a vendor directory that Go selects natively, and how its
// modules.txt must change for a temporary workspace elsewhere.
type vendorManifest struct {
	dir string // The caller's vendor directory.
	// base is the directory that the manifest's relative replacements were
	// written against: the module root, or the caller's go.work directory.
	base string
	// module marks a go mod vendor manifest, which needs the workspace header.
	module bool
	// workReplaced lists the module versions replaced by the caller's go.work;
	// an empty version replaces every version. Go compares those replacements
	// verbatim, and the temporary go.work makes them absolute.
	workReplaced map[moduleVersion]bool
	// aliases name replacement targets whose paths contain whitespace, which
	// modules.txt cannot represent: cmd/go splits its lines on spaces. Each
	// alias is a whitespace-free link inside the workspace, declared in the
	// temporary go.work, whose replacements Go compares verbatim.
	aliases      map[moduleVersion]string
	aliasTargets []string
}

type moduleVersion struct{ path, version string }

// replacementAliasDir holds the links named by vendorManifest.aliases.
const replacementAliasDir = "replacements"

// directoryReplacement reports the module, version ("" for every version) and
// path of a "# module [version] => directory" manifest line.
func directoryReplacement(line string) (fields []string, module moduleVersion, ok bool) {
	fields = strings.Fields(line)
	n := len(fields)
	if n < 4 || fields[0] != "#" || fields[n-2] != "=>" {
		return nil, moduleVersion{}, false
	}
	module.path = fields[1]
	if n == 5 {
		module.version = fields[2]
	}
	return fields, module, true
}

// assignAliases finds relative replacements whose targets contain whitespace.
// The caller's own manifest never contains those paths: a relative path from
// the module or go.work directory stays inside the shared ancestor.
func (v *vendorManifest) assignAliases(original []byte) {
	targets := map[string]string{}
	for line := range strings.SplitSeq(string(original), "\n") {
		fields, module, ok := directoryReplacement(line)
		if !ok || filepath.IsAbs(fields[len(fields)-1]) {
			continue
		}
		target := filepath.Join(v.base, fields[len(fields)-1])
		if !strings.ContainsFunc(target, unicode.IsSpace) {
			continue
		}
		alias, seen := targets[target]
		if !seen {
			alias = "./" + replacementAliasDir + "/" + strconv.Itoa(len(v.aliasTargets))
			targets[target] = alias
			v.aliasTargets = append(v.aliasTargets, target)
		}
		if v.aliases == nil {
			v.aliases = map[moduleVersion]string{}
		}
		v.aliases[module] = alias
	}
}

// declareAliases adds the aliases to the temporary go.work. Replacements of
// every version come first: go work edit drops a module's versioned
// replacements when it sets one for every version.
func (v vendorManifest) declareAliases(ctx context.Context, work string) error {
	if len(v.aliases) == 0 {
		return nil
	}
	modules := make([]moduleVersion, 0, len(v.aliases))
	for module := range v.aliases {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool {
		if (modules[i].version == "") != (modules[j].version == "") {
			return modules[i].version == ""
		}
		return modules[i].path+"@"+modules[i].version < modules[j].path+"@"+modules[j].version
	})
	args := []string{"work", "edit"}
	for _, module := range modules {
		old := module.path
		if module.version != "" {
			old += "@" + module.version
		}
		args = append(args, "-replace="+old+"="+v.aliases[module])
	}
	_, err := goTool(ctx, filepath.Dir(work), nil, append(args, work)...)
	return err
}

// linkAliases creates each alias in workspace. Go does not read replacement
// directories in vendor mode; the links keep other module queries accurate.
func (v vendorManifest) linkAliases(workspace string) error {
	for i, target := range v.aliasTargets {
		link := filepath.Join(workspace, replacementAliasDir, strconv.Itoa(i))
		if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
			return err
		}
		if err := os.Symlink(target, link); err != nil {
			return err
		}
	}
	return nil
}

func (v vendorManifest) validAliases(workspace string) bool {
	for i, target := range v.aliasTargets {
		link := filepath.Join(workspace, replacementAliasDir, strconv.Itoa(i))
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		linked, err := os.Stat(link)
		original, originalErr := os.Stat(target)
		if err != nil || originalErr != nil || !os.SameFile(linked, original) {
			return false
		}
	}
	return true
}

// fromWorkspace reports whether the caller's go.work supplies the replacement
// for path at version, using cmd/go's lookup: that exact version, then a
// replacement of every version. A manifest line without a version names the
// latter.
func (v vendorManifest) fromWorkspace(path, version string) bool {
	return v.workReplaced[moduleVersion{path, version}] || v.workReplaced[moduleVersion{path: path}]
}

// goSelectsVendor mirrors cmd/go's choice: -mod=vendor uses the directory;
// by default it needs a go directive of at least 1.14 and a modules.txt
// written for the same mode, a workspace or a single module.
func goSelectsVendor(mod, goVersion, dir string, workspace bool, replacements map[string]string) bool {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return false
	}
	switch mod {
	case "vendor":
		return true
	case "":
	default:
		return false
	}
	if goVersion == "" || compareGoVersion(goVersion, "1.14") < 0 {
		return false
	}
	manifest, err := readModuleFile(filepath.Join(dir, "modules.txt"), replacements)
	if err != nil {
		return !workspace // A module vendor directory may predate modules.txt.
	}
	return manifestIsForWorkspace(manifest) == workspace
}

// manifestIsForWorkspace reads the first line's annotations as cmd/go does.
func manifestIsForWorkspace(manifest []byte) bool {
	line, _, _ := strings.Cut(string(manifest), "\n")
	if annotations, ok := strings.CutPrefix(line, "## "); ok {
		for entry := range strings.SplitSeq(annotations, ";") {
			if strings.TrimSpace(entry) == "workspace" {
				return true
			}
		}
	}
	return false
}

// contents returns modules.txt for a workspace whose go.work is in workspace.
// Go canonicalizes a module's relative replacement against the go.work
// directory, so those paths are rewritten for this location. Replacements
// from the caller's go.work are absolute in the temporary go.work.
func (v vendorManifest) contents(original []byte, workspace string) []byte {
	manifest := string(original)
	if v.module && !manifestIsForWorkspace(original) {
		manifest = "## workspace\n" + manifest
	}
	lines := strings.Split(manifest, "\n")
	for i, line := range lines {
		fields, module, ok := directoryReplacement(line)
		n := len(fields)
		if !ok || filepath.IsAbs(fields[n-1]) {
			continue
		}
		if alias, aliased := v.aliases[module]; aliased {
			fields[n-1] = alias
			lines[i] = strings.Join(fields, " ")
			continue
		}
		path := filepath.Join(v.base, fields[n-1])
		if !v.fromWorkspace(module.path, module.version) {
			if relative, err := filepath.Rel(workspace, path); err == nil {
				path = relative
			}
			path = toDirectoryPath(path)
		}
		fields[n-1] = path
		lines[i] = strings.Join(fields, " ")
	}
	return []byte(strings.Join(lines, "\n"))
}

// toDirectoryPath and isDirectoryPath match cmd/go's canonical spelling of
// local replacement directories.
func toDirectoryPath(path string) string {
	if isDirectoryPath(path) {
		return path
	}
	return "./" + filepath.ToSlash(filepath.Clean(path))
}

func isDirectoryPath(path string) bool {
	return path == "." || strings.HasPrefix(path, "./") || strings.HasPrefix(path, `.\`) ||
		path == ".." || strings.HasPrefix(path, "../") || strings.HasPrefix(path, `..\`) ||
		strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) ||
		len(path) >= 2 && ('A' <= path[0] && path[0] <= 'Z' || 'a' <= path[0] && path[0] <= 'z') && path[1] == ':'
}

// userCacheDir locates persistent vendor workspaces; tests replace it.
var userCacheDir = os.UserCacheDir

// vendorWorkspaceLayout changes every cache key when the stored layout changes.
const vendorWorkspaceLayout = "ddtest-vendor-workspace-v3"

// vendorWorkspaceRetention bounds unused workspaces. Each one holds only
// go.work, modules.txt and links, but every vendor change creates a new one.
const vendorWorkspaceRetention = 14 * 24 * time.Hour

// vendorWorkspaceLockOpened lets tests interleave a prune with a waiting run.
var vendorWorkspaceLockOpened = func() {}

// vendorWorkspace gives work the caller's vendor tree and returns the go.work
// that Go must use. Go reads a workspace's vendor directory next to go.work,
// and reads its modules.txt outside the overlay, so that file gets its own
// workspace header. The caller's directories are recreated and every other
// entry links to the caller's tree: no file contents are copied and local
// patches stay live, while cmd/go can still traverse vendored directories.
//
// Go's build cache keys include each package directory. A content-addressed
// workspace in the user cache keeps vendored directories identical between
// runs, so unchanged dependencies are not recompiled. The returned lock keeps
// that workspace from being pruned; hold it until go test exits. Without the
// cache, links use the run's directory; without symbolic links, files are
// snapshotted there. Both run-local forms return no lock.
func vendorWorkspace(ctx context.Context, work string, vendor vendorManifest, replacements map[string]string) (string, *os.File, error) {
	debug := debugFromContext(ctx)
	source := vendor.dir
	original, err := readModuleFile(filepath.Join(source, "modules.txt"), replacements)
	if err != nil {
		return "", nil, err
	}
	tree, err := readVendorTree(source)
	if err != nil {
		return "", nil, err
	}
	vendor.assignAliases(original)
	if err := vendor.declareAliases(ctx, work); err != nil {
		return "", nil, err
	}
	stable, lock, err := stableVendorWorkspace(ctx, work, vendor, original, tree)
	if err == nil {
		retargetVendorOverlay(source, filepath.Join(filepath.Dir(stable), "vendor"), replacements)
		debug.printf("vendor workspace=cached-links")
		return stable, lock, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", nil, ctxErr
	}
	target := filepath.Join(filepath.Dir(work), "vendor")
	manifest := vendor.contents(original, filepath.Dir(work))
	if err := linkVendor(source, target, manifest, tree); err == nil && vendor.linkAliases(filepath.Dir(work)) == nil {
		retargetVendorOverlay(source, target, replacements)
		debug.printf("vendor workspace=run-links")
		return work, nil, nil
	}
	if err := os.RemoveAll(target); err != nil {
		return "", nil, err
	}
	if err := os.RemoveAll(filepath.Join(filepath.Dir(work), replacementAliasDir)); err != nil {
		return "", nil, err
	}
	debug.printf("vendor workspace=snapshot")
	return work, nil, snapshotVendor(source, target, manifest, replacements)
}

// stableVendorWorkspace returns a go.work whose directory is derived from
// everything stored there, and the shared lock that protects it. Runs that
// use a workspace hold <key>.lock shared; pruning needs it exclusively and
// never waits. Concurrent runs build privately and rename; one that loses the
// race reuses the identical winner. Unexpected contents are never modified.
func stableVendorWorkspace(ctx context.Context, work string, vendor vendorManifest, original []byte, tree []vendorEntry) (string, *os.File, error) {
	source := vendor.dir
	cache, err := userCacheDir()
	if err != nil {
		return "", nil, err
	}
	workData, err := os.ReadFile(work)
	if err != nil {
		return "", nil, err
	}
	sum, err := os.ReadFile(work + ".sum")
	hasSum := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}
	replaced := make([]string, 0, len(vendor.workReplaced))
	for module := range vendor.workReplaced {
		replaced = append(replaced, module.path+"@"+module.version)
	}
	sort.Strings(replaced)
	hash := sha256.New()
	for _, part := range [][]byte{[]byte(vendorWorkspaceLayout), []byte(source), []byte(vendor.base), []byte(strconv.FormatBool(vendor.module)), []byte(strings.Join(replaced, "\x00")), workData, sum, original} {
		fmt.Fprintf(hash, "%d:%s", len(part), part)
	}
	for _, entry := range tree {
		fmt.Fprintf(hash, "%d:%s:%t", len(entry.rel), entry.rel, entry.dir)
	}
	key := hex.EncodeToString(hash.Sum(nil))[:32]
	parent := filepath.Join(cache, "ddtest", "vendor-workspaces")
	root := filepath.Join(parent, key)
	manifest := vendor.contents(original, root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", nil, err
	}
	lock, err := holdVendorWorkspace(ctx, root+".lock")
	if err != nil {
		return "", nil, err
	}
	fail := func(err error) (string, *os.File, error) {
		lock.Close()
		return "", nil, err
	}
	if validVendorWorkspace(root, workData, manifest, tree) && vendor.validAliases(root) {
		// Only retention depends on this time. The held lock already keeps the
		// workspace from being pruned while this run uses it.
		now := time.Now()
		_ = os.Chtimes(root, now, now)
		return filepath.Join(root, "go.work"), lock, nil
	}
	staging, err := os.MkdirTemp(parent, key+".tmp-")
	if err != nil {
		return fail(err)
	}
	defer os.RemoveAll(staging) // Already gone after a successful rename.
	if err := os.WriteFile(filepath.Join(staging, "go.work"), workData, 0600); err != nil {
		return fail(err)
	}
	if hasSum {
		if err := os.WriteFile(filepath.Join(staging, "go.work.sum"), sum, 0600); err != nil {
			return fail(err)
		}
	}
	if err := linkVendor(source, filepath.Join(staging, "vendor"), manifest, tree); err != nil {
		return fail(err)
	}
	if err := vendor.linkAliases(staging); err != nil {
		return fail(err)
	}
	if err := os.Rename(staging, root); err != nil && !(validVendorWorkspace(root, workData, manifest, tree) && vendor.validAliases(root)) {
		return fail(err)
	}
	pruneVendorWorkspaces(parent, key)
	return filepath.Join(root, "go.work"), lock, nil
}

// holdVendorWorkspace takes the workspace lock shared. A pruner may remove the
// lock file while this run waits, so the held lock must still be the file at
// path; otherwise the run retries with the current file.
func holdVendorWorkspace(ctx context.Context, path string) (*os.File, error) {
	for attempt := 0; attempt < 3; attempt++ {
		lock, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		vendorWorkspaceLockOpened()
		if err := waitForSharedLock(ctx, lock); err != nil {
			lock.Close()
			return nil, err
		}
		if sameOpenFile(lock, path) {
			return lock, nil
		}
		lock.Close()
	}
	return nil, errors.New("vendor workspace lock was replaced repeatedly")
}

func sameOpenFile(file *os.File, path string) bool {
	held, heldErr := file.Stat()
	current, currentErr := os.Stat(path)
	return heldErr == nil && currentErr == nil && os.SameFile(held, current)
}

// vendorEntry is a path below the caller's vendor directory, other than the
// root modules.txt.
type vendorEntry struct {
	rel string // Relative to the vendor directory.
	dir bool   // A real directory, recreated rather than linked.
}

// readVendorTree lists the caller's vendor tree in lexical order. Real
// directories are traversed. Files and symbolic links, including links to
// directories, are entries to link: cmd/go then traverses the same directories
// in package patterns and ignores the same directory links, as it does in the
// caller's tree. Listing names reads no file contents.
func readVendorTree(source string) ([]vendorEntry, error) {
	var tree []vendorEntry
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." || rel == "modules.txt" {
			return err
		}
		tree = append(tree, vendorEntry{rel: rel, dir: entry.IsDir()})
		return nil
	})
	return tree, err
}

// linkVendor writes the workspace manifest, recreates the caller's directories
// and links every other entry. Linked files keep later edits visible; real
// directories keep vendored packages visible to wildcard patterns.
func linkVendor(source, target string, manifest []byte, tree []vendorEntry) error {
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "modules.txt"), manifest, 0600); err != nil {
		return err
	}
	for _, entry := range tree {
		path := filepath.Join(target, entry.rel)
		if entry.dir {
			if err := os.Mkdir(path, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.Symlink(filepath.Join(source, entry.rel), path); err != nil {
			return err
		}
	}
	return nil
}

// validVendorWorkspace reports whether root holds exactly what linkVendor and
// stableVendorWorkspace store: the same go.work and manifest, and the same
// directories and links. Go may add checksums to go.work.sum later.
func validVendorWorkspace(root string, work, manifest []byte, tree []vendorEntry) bool {
	if data, err := os.ReadFile(filepath.Join(root, "go.work")); err != nil || !bytes.Equal(data, work) {
		return false
	}
	vendor := filepath.Join(root, "vendor")
	if data, err := os.ReadFile(filepath.Join(vendor, "modules.txt")); err != nil || !bytes.Equal(data, manifest) {
		return false
	}
	next := 0
	err := filepath.WalkDir(vendor, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(vendor, path)
		if err != nil || rel == "." || rel == "modules.txt" {
			return err
		}
		if next == len(tree) || tree[next].rel != rel || tree[next].dir != entry.IsDir() ||
			!entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
			return errors.New("unexpected vendor workspace entry")
		}
		next++
		return nil
	})
	return err == nil && next == len(tree)
}

// pruneVendorWorkspaces removes workspaces unused for the retention period,
// their locks and abandoned staging directories. Removal deletes links, never
// the caller's vendored files.
func pruneVendorWorkspaces(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-vendorWorkspaceRetention)
	for _, entry := range entries {
		name := entry.Name()
		key, isLock := strings.CutSuffix(name, ".lock")
		if name == keep || key == keep {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		switch {
		case strings.Contains(name, ".tmp-"):
			// Staging directories are private to the run that creates them.
			_ = os.RemoveAll(filepath.Join(parent, name))
		case isLock:
			if _, err := os.Lstat(filepath.Join(parent, key)); errors.Is(err, os.ErrNotExist) {
				removeUnusedVendorWorkspace(parent, key, cutoff)
			}
		case entry.IsDir():
			removeUnusedVendorWorkspace(parent, name, cutoff)
		}
	}
}

// removeUnusedVendorWorkspace removes a workspace only while no run holds it.
// A run that reused it after the directory scan refreshed its time, so the
// time is checked again under the exclusive lock.
func removeUnusedVendorWorkspace(parent, key string, cutoff time.Time) {
	path := filepath.Join(parent, key+".lock")
	lock, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer lock.Close()
	if tryLockFile(lock, true) != nil || !sameOpenFile(lock, path) {
		return
	}
	root := filepath.Join(parent, key)
	if info, err := os.Lstat(root); err == nil && !info.ModTime().Before(cutoff) {
		return
	}
	if err := os.RemoveAll(root); err != nil {
		return
	}
	if os.Remove(path) != nil {
		// Windows cannot remove an open file, including this handle. A run that
		// opens it meanwhile finds no workspace and rebuilds one; a file that
		// another run still has open stays for a later prune.
		lock.Close()
		_ = os.Remove(path)
	}
}

// retargetVendorOverlay moves the caller's vendor overlay entries, including
// virtual files and deletions, to the workspace vendor directory. Go reads
// modules.txt outside the overlay, so that entry is already in the manifest.
func retargetVendorOverlay(source, target string, replacements map[string]string) {
	moved := map[string]string{}
	for path, backing := range replacements {
		rel, err := filepath.Rel(source, path)
		if err == nil && rel != "modules.txt" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			moved[filepath.Join(target, rel)] = backing
		}
	}
	for path, backing := range moved {
		replacements[path] = backing
	}
}

// snapshotVendor is the fallback where symbolic links are unavailable. It
// preserves patched sources; hard links avoid copying large vendor trees and a
// copy is used across filesystems. The modules.txt copy is always independent
// because Go reads that file outside its overlay filesystem.
func snapshotVendor(source, target string, manifest []byte, replacements map[string]string) error {
	// Include virtual files added by an overlay, not just files visited on disk.
	for path, backing := range replacements {
		rel, err := filepath.Rel(source, path)
		if err == nil && rel != "modules.txt" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			replacements[filepath.Join(target, rel)] = backing
		}
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		logical := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(logical, 0700)
		}
		backing := path
		if replacement, ok := replacements[path]; ok {
			if replacement == "" {
				return nil
			}
			backing = replacement
			// Retarget the user's source overlay for the temporary vendor directory.
			if rel != "modules.txt" {
				replacements[logical] = replacement
			}
		}
		if rel == "modules.txt" {
			return os.WriteFile(logical, manifest, 0600)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(backing)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil // Go does not traverse symlink directories in package scans.
			}
			backing, err = filepath.EvalSymlinks(backing)
			if err != nil {
				return err
			}
		}
		if err := os.Link(backing, logical); err == nil {
			return nil
		}
		input, err := os.Open(backing)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(logical, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
