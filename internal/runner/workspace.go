package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// workspaceFile follows Go's workspace discovery without changing process-wide
// environment. Each child command receives the task's temporary GOWORK value.
func workspaceFile(dir string) string {
	if work := os.Getenv("GOWORK"); work != "" && work != "auto" {
		if work == "off" {
			return ""
		}
		return work
	}
	for {
		path := filepath.Join(dir, "go.work")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// provideMiniWorkspace preserves every use/replace directive and each module's
// own language version. Mini is a main module in the temporary workspace, so
// importing it adds no requirements to any client module.
func provideMiniWorkspace(ctx context.Context, dir, work, temp string, replacements map[string]string) (string, error) {
	if !filepath.IsAbs(work) {
		return "", fmt.Errorf("GOWORK must name an absolute path: %s", work)
	}
	data, err := readModuleFile(work, replacements)
	if err != nil {
		return "", err
	}
	target := filepath.Join(temp, "go.work")
	if err := os.WriteFile(target, data, 0600); err != nil {
		return "", err
	}
	if err := copyModuleFile(work+".sum", target+".sum", replacements); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	out, err := goTool(ctx, dir, nil, "work", "edit", "-json", target)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Go      string
		GoDebug []struct{ Key, Value string }
		Use     []struct{ DiskPath string }
		Replace []struct {
			Old, New struct{ Path, Version string }
		}
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return "", fmt.Errorf("parse workspace edit JSON (%d bytes): %w", len(out), err)
	}
	base := filepath.Dir(work)
	args := []string{"work", "edit"}
	if parsed.Go == "" {
		args = append(args, "-go=1.21")
	}
	for _, use := range parsed.Use {
		if !filepath.IsAbs(use.DiskPath) {
			args = append(args, "-dropuse="+use.DiskPath, "-use="+filepath.Join(base, use.DiskPath))
		}
	}
	for _, replace := range parsed.Replace {
		if replace.New.Version == "" && !filepath.IsAbs(replace.New.Path) {
			old := replace.Old.Path
			if replace.Old.Version != "" {
				old += "@" + replace.Old.Version
			}
			args = append(args, "-replace="+old+"="+filepath.Join(base, replace.New.Path))
		}
	}

	// Use the effective local Mini sources as a main module. A carrier requiring
	// Mini would force Go to load unrelated dependency versions that a workspace
	// normally resolves directly from its main modules.
	// Resolve against the copied workspace before introducing Mini. This keeps
	// existing requirements, remote replacements and conflicting replacements
	// subject to Go's own module selection rules.
	if len(args) > 2 {
		if _, err := goTool(ctx, dir, nil, append(args, target)...); err != nil {
			return "", err
		}
	}
	miniRoot, err := selectedWorkspaceMini(ctx, dir, target, temp, replacements)
	if err != nil {
		return "", err
	}
	selected := miniRoot != ""
	workspaceReplacement := false
	for _, replace := range parsed.Replace {
		if replace.Old.Path != miniModule {
			continue
		}
		workspaceReplacement = true
		if !selected {
			miniRoot, err = workspaceReplacementRoot(ctx, dir, base, replace.New.Path, replace.New.Version)
			if err != nil {
				return "", err
			}
		}
		old := replace.Old.Path
		if replace.Old.Version != "" {
			old += "@" + replace.Old.Version
		}
		args = append(args, "-dropreplace="+old)
	}
	for i, use := range parsed.Use {
		root := use.DiskPath
		if !filepath.IsAbs(root) {
			root = filepath.Join(base, root)
		}
		moduleData, e := readModuleFile(filepath.Join(root, "go.mod"), replacements)
		if e != nil {
			return "", e
		}
		if modulePath(moduleData) == miniModule {
			miniRoot = root
			break
		}
		if workspaceReplacement || selected {
			continue
		}
		copy := filepath.Join(temp, fmt.Sprintf("client-%d.mod", i))
		if err := os.WriteFile(copy, moduleData, 0600); err != nil {
			return "", err
		}
		jsonData, e := goTool(ctx, root, nil, "mod", "edit", "-json", "-modfile="+copy)
		if e != nil {
			return "", e
		}
		var client struct {
			Replace []struct {
				Old, New struct{ Path, Version string }
			}
		}
		if err := json.Unmarshal([]byte(jsonData), &client); err != nil {
			return "", fmt.Errorf("parse client edit JSON (%d bytes): %w", len(jsonData), err)
		}
		for _, replace := range client.Replace {
			if replace.Old.Path != miniModule {
				continue
			}
			candidate, err := workspaceReplacementRoot(ctx, dir, root, replace.New.Path, replace.New.Version)
			if err != nil {
				return "", err
			}
			if miniRoot != "" && filepath.Clean(miniRoot) != filepath.Clean(candidate) {
				return "", fmt.Errorf("conflicting workspace replacements for %s", miniModule)
			}
			miniRoot = candidate
		}
	}
	if miniRoot == "" {
		var cliVersion string
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path == miniModule && publishedVersion(info.Main.Version) {
			cliVersion = info.Main.Version
		}
		cache, err := goTool(ctx, dir, nil, "env", "GOMODCACHE")
		if err != nil {
			return "", err
		}
		_, source, _, _ := runtime.Caller(0)
		miniRoot = miniSourceRoot(source, strings.TrimSpace(cache), cliVersion)
		if miniRoot == "" && cliVersion != "" {
			miniRoot, err = workspaceReplacementRoot(ctx, dir, base, miniModule, cliVersion)
			if err != nil {
				return "", err
			}
		}
	}
	if !validMiniSource(miniRoot) {
		return "", fmt.Errorf("Mini sources unavailable for workspace provisioning")
	}
	miniData, err := readModuleFile(filepath.Join(miniRoot, "go.mod"), replacements)
	if err != nil {
		return "", err
	}
	if required := moduleDirective(miniData, "go"); compareGoVersion(required, parsed.Go) > 0 {
		args = append(args, "-go="+required)
		// A workspace's go directive also controls program compatibility defaults.
		// Preserve those defaults if a selected runtime needs a newer workspace.
		hasDefault := false
		for _, setting := range parsed.GoDebug {
			hasDefault = hasDefault || setting.Key == "default"
		}
		if !hasDefault && parsed.Go != "" {
			parts := strings.Split(parsed.Go, ".")
			if len(parts) >= 2 {
				args = append(args, "-godebug=default=go"+strings.Join(parts[:2], "."))
			}
		}
	}
	args = append(args, "-use="+miniRoot, target)
	_, err = goTool(ctx, dir, nil, args...)
	return target, err
}

// selectedWorkspaceMini probes only the runtime module. Its checksum writes
// belong to the copied workspace; readonly protects the client module files.
func selectedWorkspaceMini(ctx context.Context, dir, work, temp string, replacements map[string]string) (result string, err error) {
	phase := debugFromContext(ctx).start("resolve workspace runtime")
	defer func() { phase.finish(err) }()
	args := []string{"list", "-m", "-e", "-json", "-mod=readonly"}
	if len(replacements) != 0 {
		data, err := json.Marshal(Overlay{Replace: replacements})
		if err != nil {
			return "", err
		}
		path := filepath.Join(temp, "workspace-probe-overlay.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			return "", err
		}
		args = append(args, "-overlay="+path)
	}
	args = append(args, miniModule)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GOWORK="+work, "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve workspace Mini: %w", err)
	}
	// With multiple workspace main modules, go list -m -e can omit an unknown
	// explicit module completely: exit 0, no JSON and no stderr. Provision it.
	if len(bytes.TrimSpace(out)) == 0 {
		return "", nil
	}
	var module struct {
		Path, Version, Dir string
		Replace            *struct{ Path, Version, Dir string }
		Error              *struct{ Err string }
	}
	if err := json.Unmarshal(out, &module); err != nil {
		return "", fmt.Errorf("parse workspace module JSON: %w", err)
	}
	if module.Error != nil {
		if strings.Contains(module.Error.Err, "not a known dependency") {
			return "", nil
		}
		return "", fmt.Errorf("resolve workspace Mini: %s", module.Error.Err)
	}
	if module.Replace != nil {
		if module.Replace.Dir != "" {
			return module.Replace.Dir, nil
		}
		return workspaceReplacementRoot(ctx, dir, dir, module.Replace.Path, module.Replace.Version)
	}
	if module.Dir != "" {
		return module.Dir, nil
	}
	return workspaceReplacementRoot(ctx, dir, dir, module.Path, module.Version)
}

func workspaceReplacementRoot(ctx context.Context, dir, base, path, version string) (string, error) {
	if version == "" {
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		return path, nil
	}
	// Download outside all client modules and workspaces. Explicit module
	// queries populate only GOMODCACHE, without writing caller checksum files.
	scratch, err := os.MkdirTemp("", "ddtest-module-download-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", path+"@"+version)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = scratch
	cmd.Env = append(cmd.Environ(), "GOWORK=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	phase := debugFromContext(ctx).start("go mod download")
	out, err := cmd.Output()
	phase.finish(err)
	if err != nil {
		return "", fmt.Errorf("download workspace Mini: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var module struct{ Dir string }
	if err := json.Unmarshal(out, &module); err != nil {
		return "", fmt.Errorf("parse workspace module JSON (%d bytes): %w", len(out), err)
	}
	return module.Dir, nil
}
