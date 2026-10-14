package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The runtime archive holds the module files and the non-test sources of the
// runtime's directories, and the same sources give the same bytes.
func TestRuntimeArchiveContents(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	first, err := runtimeArchive(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtimeArchive(root)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("archive not reproducible: %v", err)
	}
	files := archiveFiles(t, first)
	for _, want := range []string{"go.mod", "LICENSE", "testopt/testopt.go", "internal/toolwrap/toolwrap.go", "internal/bundle/bundle.go"} {
		if !files[want] {
			t.Errorf("archive lacks %s", want)
		}
	}
	// Every license text of the incorporated sources travels with them.
	for _, dir := range runtimeDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			upper := strings.ToUpper(entry.Name())
			if rel, _ := filepath.Rel(root, path); (strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "NOTICE")) &&
				!strings.Contains(filepath.ToSlash(rel), "/testdata/") && !files[filepath.ToSlash(rel)] {
				t.Errorf("archive lacks %s", filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for name := range files {
		if strings.HasSuffix(name, "_test.go") || strings.Contains(name, "/testdata/") || strings.HasPrefix(name, "cmd/") {
			t.Errorf("archive holds %s", name)
		}
	}
}

// Every file that Go compiles for a runtime package is in the archive, and no
// runtime package lies outside it. Tests can target any platform that Go
// supports, not only those of the release executables.
func TestRuntimeArchiveCoversEveryPlatform(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	archive, err := runtimeArchive(root)
	if err != nil {
		t.Fatal(err)
	}
	files := archiveFiles(t, archive)
	patterns := make([]string, 0, len(runtimeDirs))
	for _, dir := range runtimeDirs {
		patterns = append(patterns, "./"+dir+"/...")
	}
	ports, err := exec.Command("go", "tool", "dist", "list").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range strings.Fields(string(ports)) {
		goos, goarch, _ := strings.Cut(target, "/")
		cmd := exec.Command("go", append([]string{"list", "-deps", "-json=ImportPath,Dir,Standard,GoFiles,CgoFiles,SFiles,HFiles,CFiles,EmbedFiles"}, patterns...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0", "GOFLAGS=", "GOWORK=off")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s: go list: %v", target, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(out))
		for decoder.More() {
			var p struct {
				ImportPath, Dir                                       string
				Standard                                              bool
				GoFiles, CgoFiles, SFiles, HFiles, CFiles, EmbedFiles []string
			}
			if err := decoder.Decode(&p); err != nil {
				t.Fatal(err)
			}
			if p.Standard {
				continue
			}
			rel, err := filepath.Rel(root, p.Dir)
			if err != nil || !strings.HasPrefix(p.ImportPath, module+"/") {
				t.Fatalf("%s: package %s outside the module", target, p.ImportPath)
			}
			for _, group := range [][]string{p.GoFiles, p.CgoFiles, p.SFiles, p.HFiles, p.CFiles, p.EmbedFiles} {
				for _, name := range group {
					if path := filepath.ToSlash(filepath.Join(rel, name)); !files[path] {
						t.Errorf("%s: archive lacks %s", target, path)
					}
				}
			}
		}
	}
}

func archiveFiles(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, file := range reader.File {
		files[file.Name] = true
	}
	return files
}
