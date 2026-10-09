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
	// workReplaced lists modules replaced by the caller's go.work. Go compares
	// those replacements verbatim, and the temporary go.work makes them absolute.
	workReplaced map[string]bool
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
		// "# module [version] => directory" names a directory replacement.
		fields := strings.Fields(line)
		n := len(fields)
		if n < 4 || fields[0] != "#" || fields[n-2] != "=>" || filepath.IsAbs(fields[n-1]) {
			continue
		}
		path := filepath.Join(v.base, fields[n-1])
		if !v.workReplaced[fields[1]] {
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
const vendorWorkspaceLayout = "ddtest-vendor-workspace-v1"

// vendorWorkspaceRetention bounds unused workspaces. Each one holds only
// go.work, modules.txt and links, but every vendor change creates a new one.
const vendorWorkspaceRetention = 14 * 24 * time.Hour

// vendorWorkspaceLockOpened lets tests interleave a prune with a waiting run.
var vendorWorkspaceLockOpened = func() {}

// vendorWorkspace gives work the caller's vendor tree and returns the go.work
// that Go must use. Go reads a workspace's vendor directory next to go.work,
// and reads its modules.txt outside the overlay, so that file gets its own
// workspace header. Every other top-level entry links to the caller's tree:
// nothing is copied and local patches stay live.
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
	entries, err := os.ReadDir(source)
	if err != nil {
		return "", nil, err
	}
	stable, lock, err := stableVendorWorkspace(ctx, work, vendor, original, entries)
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
	if err := linkVendor(source, target, manifest, entries); err == nil {
		retargetVendorOverlay(source, target, replacements)
		debug.printf("vendor workspace=run-links")
		return work, nil, nil
	}
	if err := os.RemoveAll(target); err != nil {
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
func stableVendorWorkspace(ctx context.Context, work string, vendor vendorManifest, original []byte, entries []fs.DirEntry) (string, *os.File, error) {
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
	for path := range vendor.workReplaced {
		replaced = append(replaced, path)
	}
	sort.Strings(replaced)
	hash := sha256.New()
	for _, part := range [][]byte{[]byte(vendorWorkspaceLayout), []byte(source), []byte(vendor.base), []byte(strconv.FormatBool(vendor.module)), []byte(strings.Join(replaced, "\x00")), workData, sum, original} {
		fmt.Fprintf(hash, "%d:%s", len(part), part)
	}
	for _, entry := range entries {
		fmt.Fprintf(hash, "%d:%s:%d", len(entry.Name()), entry.Name(), entry.Type())
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
	if validVendorWorkspace(root, workData, source, manifest, entries) {
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
	if err := linkVendor(source, filepath.Join(staging, "vendor"), manifest, entries); err != nil {
		return fail(err)
	}
	if err := os.Rename(staging, root); err != nil && !validVendorWorkspace(root, workData, source, manifest, entries) {
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

// linkVendor writes the workspace manifest and links every other top-level
// entry, so nested links and later source edits behave as in the caller's tree.
func linkVendor(source, target string, manifest []byte, entries []fs.DirEntry) error {
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "modules.txt"), manifest, 0600); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "modules.txt" {
			continue
		}
		if err := os.Symlink(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// validVendorWorkspace reports whether root holds exactly what linkVendor and
// stableVendorWorkspace store. Go may add checksums to go.work.sum later.
func validVendorWorkspace(root string, work []byte, source string, manifest []byte, entries []fs.DirEntry) bool {
	if data, err := os.ReadFile(filepath.Join(root, "go.work")); err != nil || !bytes.Equal(data, work) {
		return false
	}
	vendor := filepath.Join(root, "vendor")
	if data, err := os.ReadFile(filepath.Join(vendor, "modules.txt")); err != nil || !bytes.Equal(data, manifest) {
		return false
	}
	present, err := os.ReadDir(vendor)
	if err != nil {
		return false
	}
	links := 0
	for _, entry := range entries {
		if entry.Name() == "modules.txt" {
			continue
		}
		links++
		link := filepath.Join(vendor, entry.Name())
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		// Compare files, not link text: Windows can report another spelling.
		linked, err := os.Stat(link)
		original, originalErr := os.Stat(filepath.Join(source, entry.Name()))
		if err != nil || originalErr != nil || !os.SameFile(linked, original) {
			return false
		}
	}
	return len(present) == links+1
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
