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

func provideMiniVendorWorkspace(ctx context.Context, dir, root, temp string, replacements map[string]string) (string, error) {
	source, err := moduleWorkspaceInput(ctx, dir, root, temp, replacements)
	if err != nil {
		return "", err
	}
	work, err := provideMiniWorkspace(ctx, dir, source, temp, replacements)
	if err != nil {
		return "", err
	}
	return vendorWorkspace(ctx, work, filepath.Join(root, "vendor"), replacements)
}

// userCacheDir locates persistent vendor workspaces; tests replace it.
var userCacheDir = os.UserCacheDir

// vendorWorkspaceLayout changes every cache key when the stored layout changes.
const vendorWorkspaceLayout = "ddtest-vendor-workspace-v1"

// vendorWorkspaceRetention bounds unused workspaces. Each one holds only
// go.work, modules.txt and links, but every vendor change creates a new one.
const vendorWorkspaceRetention = 14 * 24 * time.Hour

// vendorWorkspace gives work the caller's vendor tree and returns the go.work
// that Go must use. Go reads a workspace's vendor directory next to go.work,
// and reads its modules.txt outside the overlay, so that file gets its own
// workspace header. Every other top-level entry links to the caller's tree:
// nothing is copied and local patches stay live.
//
// Go's build cache keys include each package directory. A content-addressed
// workspace in the user cache keeps vendored directories identical between
// runs, so unchanged dependencies are not recompiled. Without that cache the
// links use the run's directory; without symbolic links, files are snapshotted.
func vendorWorkspace(ctx context.Context, work, source string, replacements map[string]string) (string, error) {
	debug := debugFromContext(ctx)
	manifest, err := readModuleFile(filepath.Join(source, "modules.txt"), replacements)
	if err != nil {
		return "", err
	}
	if !bytes.HasPrefix(manifest, []byte("## workspace")) {
		manifest = append([]byte("## workspace\n"), manifest...)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return "", err
	}
	if stable, err := stableVendorWorkspace(work, source, manifest, entries); err == nil {
		retargetVendorOverlay(source, filepath.Join(filepath.Dir(stable), "vendor"), replacements)
		debug.printf("vendor workspace=cached-links")
		return stable, nil
	}
	target := filepath.Join(filepath.Dir(work), "vendor")
	if err := linkVendor(source, target, manifest, entries); err == nil {
		retargetVendorOverlay(source, target, replacements)
		debug.printf("vendor workspace=run-links")
		return work, nil
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	debug.printf("vendor workspace=snapshot")
	return work, snapshotVendor(source, target, replacements)
}

// stableVendorWorkspace returns a go.work whose directory is derived from
// everything stored there. Concurrent runs build privately and rename; a run
// that loses the race reuses the identical winner. An existing directory with
// unexpected contents is never modified.
func stableVendorWorkspace(work, source string, manifest []byte, entries []fs.DirEntry) (string, error) {
	cache, err := userCacheDir()
	if err != nil {
		return "", err
	}
	workData, err := os.ReadFile(work)
	if err != nil {
		return "", err
	}
	sum, err := os.ReadFile(work + ".sum")
	hasSum := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	hash := sha256.New()
	for _, part := range [][]byte{[]byte(vendorWorkspaceLayout), []byte(source), workData, sum, manifest} {
		fmt.Fprintf(hash, "%d:%s", len(part), part)
	}
	for _, entry := range entries {
		fmt.Fprintf(hash, "%d:%s:%d", len(entry.Name()), entry.Name(), entry.Type())
	}
	key := hex.EncodeToString(hash.Sum(nil))[:32]
	parent := filepath.Join(cache, "ddtest", "vendor-workspaces")
	root := filepath.Join(parent, key)
	if validVendorWorkspace(root, workData, source, manifest, entries) {
		now := time.Now()
		_ = os.Chtimes(root, now, now) // Retention counts from the last use.
		return filepath.Join(root, "go.work"), nil
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(parent, key+".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging) // Already gone after a successful rename.
	if err := os.WriteFile(filepath.Join(staging, "go.work"), workData, 0600); err != nil {
		return "", err
	}
	if hasSum {
		if err := os.WriteFile(filepath.Join(staging, "go.work.sum"), sum, 0600); err != nil {
			return "", err
		}
	}
	if err := linkVendor(source, filepath.Join(staging, "vendor"), manifest, entries); err != nil {
		return "", err
	}
	if err := os.Rename(staging, root); err != nil && !validVendorWorkspace(root, workData, source, manifest, entries) {
		return "", err
	}
	pruneVendorWorkspaces(parent, key)
	return filepath.Join(root, "go.work"), nil
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

// pruneVendorWorkspaces removes unused workspaces and abandoned staging
// directories. Removal deletes links, never the caller's vendored files.
func pruneVendorWorkspaces(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-vendorWorkspaceRetention)
	for _, entry := range entries {
		if entry.Name() == keep {
			continue
		}
		if info, err := entry.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(parent, entry.Name()))
		}
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
func snapshotVendor(source, target string, replacements map[string]string) error {
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
			data, err := os.ReadFile(backing)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(string(data), "## workspace") {
				data = append([]byte("## workspace\n"), data...)
			}
			return os.WriteFile(logical, data, 0600)
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
