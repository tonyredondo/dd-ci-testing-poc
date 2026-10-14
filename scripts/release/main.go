// Command release builds ddto's release executables, one per platform. Each
// carries the Mini runtime's sources and the tool wrapper executable for its
// platform (see internal/bundle), so that users need neither the module's
// sources nor a build of ddto or its wrapper. Run it from the module root:
//
//	go run ./scripts/release [-o dist] [-target linux/amd64,darwin/arm64]
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/bundle"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/toolwrap"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/version"
)

const module = "github.com/tonyredondo/dd-ci-testing-poc"

// targets are the platforms of GitHub's hosted runners and of most developers.
var targets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

// runtimeDirs hold every package that the Mini runtime and ddto's
// instrumentation import. cmd/ddto, tests, docs and scripts stay out.
var runtimeDirs = []string{"internal", "propagation", "testopt"}

// runtimeFiles are the module files that Go needs to compile those packages.
var runtimeFiles = []string{"go.mod", "LICENSE", "NOTICE"}

func main() {
	out := flag.String("o", "dist", "output directory")
	list := flag.String("target", strings.Join(targets, ","), "comma-separated GOOS/GOARCH platforms")
	flag.Parse()
	root, err := moduleRoot()
	must(err)
	archive, err := runtimeArchive(root)
	must(err)
	temp, err := os.MkdirTemp("", "ddto-release-")
	must(err)
	defer os.RemoveAll(temp)
	must(os.WriteFile(filepath.Join(temp, bundle.RuntimeArchive), archive, 0600))
	must(os.MkdirAll(*out, 0755))
	for _, target := range strings.Split(*list, ",") {
		goos, goarch, ok := strings.Cut(target, "/")
		if !ok {
			must(fmt.Errorf("target %q is not GOOS/GOARCH", target))
		}
		path, err := build(root, temp, *out, goos, goarch)
		must(err)
		fmt.Println(path)
	}
}

func moduleRoot() (string, error) {
	gomod, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		return "", err
	}
	root := filepath.Dir(strings.TrimSpace(string(gomod)))
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !bytes.HasPrefix(data, []byte("module "+module+"\n")) {
		return "", fmt.Errorf("run from the %s module", module)
	}
	return root, nil
}

// runtimeArchive zips the runtime's sources: the module files and the non-test
// sources of runtimeDirs, without testdata. Entries are sorted and carry no
// times, so the same sources give the same archive and the same cache key.
func runtimeArchive(root string) ([]byte, error) {
	names := slices.Clone(runtimeFiles)
	for _, dir := range runtimeDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() {
				if name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(name) {
			case ".go", ".s", ".h", ".c":
			default:
				return nil
			}
			if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			names = append(names, filepath.ToSlash(rel))
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(names)
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)}
		w, err := archive.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// build writes ddto-GOOS-GOARCH to out. The wrapper comes from the sources
// that ddto would build, so its key matches; -overlay supplies it and the
// runtime archive to internal/bundle without writing to the checkout.
func build(root, temp, out, goos, goarch string) (string, error) {
	suffix := ""
	if goos == "windows" {
		suffix = ".exe"
	}
	src := filepath.Join(temp, "toolwrap-src")
	if err := os.RemoveAll(src); err != nil {
		return "", err
	}
	files, _ := toolwrap.Sources(version.BuildLogPrefix)
	for name, data := range files {
		path := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			return "", err
		}
	}
	wrapper := filepath.Join(temp, "toolwrap-"+goos+"-"+goarch)
	env := append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
	if err := run(src, env, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", wrapper, "."); err != nil {
		return "", err
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {
		filepath.Join(root, "internal", "bundle", bundle.RuntimeArchive):  filepath.Join(temp, bundle.RuntimeArchive),
		filepath.Join(root, "internal", "bundle", bundle.ToolWrapperFile): wrapper,
	}})
	if err != nil {
		return "", err
	}
	overlayFile := filepath.Join(temp, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0600); err != nil {
		return "", err
	}
	path, err := filepath.Abs(filepath.Join(out, "ddto-"+goos+"-"+goarch+suffix))
	if err != nil {
		return "", err
	}
	err = run(root, env, "go", "build", "-trimpath", "-tags=ddto_release", "-ldflags=-s -w", "-overlay="+overlayFile, "-o", path, "./cmd/ddto")
	return path, err
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}
