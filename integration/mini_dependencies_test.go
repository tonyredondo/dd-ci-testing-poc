package integration

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMiniConsumerAddsOnlyOwnModule(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	client := t.TempDir()
	mod := []byte("module example.com/mini-consumer\n\ngo 1.26.0\n\nrequire github.com/tonyredondo/dd-ci-testing-poc v0.0.0\n\nreplace github.com/tonyredondo/dd-ci-testing-poc => " + strconv.Quote(filepath.ToSlash(root)) + "\n")
	if err := os.WriteFile(filepath.Join(client, "go.mod"), mod, 0600); err != nil {
		t.Fatal(err)
	}
	source := []byte("package main\nimport _ \"github.com/tonyredondo/dd-ci-testing-poc/testopt\"\nfunc main() {}\n")
	if err := os.WriteFile(filepath.Join(client, "main.go"), source, 0600); err != nil {
		t.Fatal(err)
	}
	env := testEnv("GOWORK=off", "GOPROXY=off")
	if stdout, stderr, code := command(t, client, env, "go", "mod", "tidy"); code != 0 {
		t.Fatalf("client tidy failed: %s%s", stdout, stderr)
	}
	stdout, stderr, code := command(t, client, env, "go", "mod", "edit", "-json")
	if code != 0 {
		t.Fatalf("client go.mod failed: %s%s", stdout, stderr)
	}
	var parsed struct{ Require []struct{ Path string } }
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Require) != 1 || parsed.Require[0].Path != "github.com/tonyredondo/dd-ci-testing-poc" {
		t.Fatalf("consumer requires extra modules: %+v", parsed.Require)
	}
	stdout, stderr, code = command(t, client, env, "go", "list", "-deps", "-json", ".")
	if code != 0 {
		t.Fatalf("client graph failed: %s%s", stdout, stderr)
	}
	decoder := json.NewDecoder(strings.NewReader(stdout))
	for {
		var pkg struct {
			ImportPath string
			Standard   bool
			Module     *struct{ Path string }
		}
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		if !pkg.Standard && pkg.Module != nil && pkg.Module.Path != "github.com/tonyredondo/dd-ci-testing-poc" && pkg.Module.Path != "example.com/mini-consumer" {
			t.Fatalf("external runtime package %s from %s", pkg.ImportPath, pkg.Module.Path)
		}
	}
}
