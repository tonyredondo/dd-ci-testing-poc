package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCoverageSelection(t *testing.T) {
	root := t.TempDir()
	client := goPackage{Dir: root, ImportPath: "example.com/client"}
	helper := goPackage{Dir: filepath.Join(root, "helpers"), ImportPath: "example.com/client/helpers"}
	for _, tc := range []struct {
		name    string
		args    []string
		env     string
		changed []goPackage
		want    bool
	}{
		{"off", nil, "", []goPackage{helper}, false},
		{"test files only", []string{"-cover"}, "", nil, false},
		{"default excludes helper", []string{"-cover"}, "", []goPackage{helper}, false},
		{"default includes root", []string{"-cover"}, "", []goPackage{client}, true},
		{"root pattern", []string{"-coverpkg=."}, "", []goPackage{helper}, false},
		{"recursive pattern", []string{"-coverpkg=./..."}, "", []goPackage{helper}, true},
		{"import pattern", []string{"-coverpkg=example.com/client/..."}, "", []goPackage{client}, true},
		{"outside relative tree", []string{"-coverpkg=./..."}, "", []goPackage{{Dir: filepath.Join(filepath.Dir(root), "goroot", "src", "testing"), ImportPath: "testing"}}, false},
		{"other pattern", []string{"-coverpkg=example.com/other/..."}, "", []goPackage{helper}, false},
		{"GOFLAGS", nil, "-coverpkg=./...", []goPackage{helper}, true},
		{"empty coverpkg", []string{"-coverpkg="}, "", []goPackage{client}, true},
		{"profile", []string{"-coverprofile=result.out"}, "", []goPackage{client}, true},
		{"custom args", []string{"-args", "-coverpkg=./..."}, "", []goPackage{helper}, false},
		{"false", []string{"-cover=false"}, "", []goPackage{client}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseOptions(tc.args, tc.env)
			if err != nil {
				t.Fatal(err)
			}
			if got := needsCoverOverlay(root, opts, []goPackage{client}, tc.changed); got != tc.want {
				t.Fatalf("activation=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCoverInputOrderingAndGeneratedExclusion(t *testing.T) {
	for _, assigned := range []bool{true, false} {
		t.Run(map[bool]string{true: "assigned", false: "separate"}[assigned], func(t *testing.T) {
			dir := t.TempDir()
			original := filepath.Join(dir, "original.go")
			actual := filepath.Join(dir, "rewritten.go")
			logical := filepath.Join(dir, "zz_dd_ci_testify.go")
			wrapper := filepath.Join(dir, "wrapper.go")
			if err := os.WriteFile(wrapper, []byte("package helper\nfunc adapter() {}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			overlay := filepath.Join(dir, "overlay.json")
			data, _ := json.Marshal(Overlay{Replace: map[string]string{original: actual, logical: wrapper}, CoverExclude: []string{logical}})
			if err := os.WriteFile(overlay, data, 0600); err != nil {
				t.Fatal(err)
			}
			outList := filepath.Join(dir, "outputs.txt")
			outputs := []string{filepath.Join(dir, "vars.go"), filepath.Join(dir, "original.cover.go"), filepath.Join(dir, "wrapper.cover.go")}
			if err := os.WriteFile(outList, []byte(strings.Join(outputs, "\n")), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"cover", "-pkgcfg=config"}
			if assigned {
				args = append(args, "-outfilelist="+outList)
			} else {
				args = append(args, "-outfilelist", outList)
			}
			args = append(args, original, logical)
			saved := append([]string(nil), args...)
			got, finish, cleanup, err := prepareCoverInputs(overlay, args)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			if !reflect.DeepEqual(args, saved) {
				t.Fatal("mutated caller arguments")
			}
			if got[len(got)-1] != actual || len(got) != len(args)-1 {
				t.Fatalf("inputs=%v", got)
			}
			list := got[2]
			if assigned {
				list = strings.TrimPrefix(list, "-outfilelist=")
			} else {
				list = got[3]
			}
			data, err = os.ReadFile(list)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != strings.Join(outputs[:2], "\n")+"\n" {
				t.Fatalf("outputs=%q", data)
			}
			if err := finish(); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(outputs[2])
			if err != nil || string(data) != "package helper\nfunc adapter() {}\n" {
				t.Fatalf("wrapper=%q, %v", data, err)
			}
			cleanup()
			if _, err := os.Stat(list); !os.IsNotExist(err) {
				t.Fatalf("temporary output list remains: %v", err)
			}
		})
	}
}

func TestCoverToolForwardingAndErrors(t *testing.T) {
	output, err := exec.Command("go", "env", "GOTOOLDIR").Output()
	if err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if strings.HasSuffix(os.Args[0], ".exe") {
		suffix = ".exe"
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	data, _ := json.Marshal(Overlay{Replace: map[string]string{}})
	if err := os.WriteFile(overlay, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"compile", "cover"} {
		executable := filepath.Join(strings.TrimSpace(string(output)), tool+suffix)
		want, err := exec.Command(executable, "-V=full").CombinedOutput()
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		code := RunCoverTool(context.Background(), overlay, []string{executable, "-V=full"}, nil, &stdout, &stderr)
		if tool == "cover" {
			want = []byte(appendCoverIdentity(string(want), coverFingerprint(Overlay{})))
		}
		if code != 0 || !bytes.Equal(stdout.Bytes(), want) || stderr.Len() != 0 {
			t.Fatalf("%s identity changed: %d %s %s", tool, code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := RunCoverTool(context.Background(), "missing-overlay", nil, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("missing tool exit=%d", code)
	}
	if code := RunCoverTool(context.Background(), "missing-overlay", []string{filepath.Join(strings.TrimSpace(string(output)), "cover"+suffix), "-mode=set", "file.go"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("missing overlay exit=%d", code)
	}
	// A forwarded tool failure must retain its native exit status.
	compile := filepath.Join(strings.TrimSpace(string(output)), "compile"+suffix)
	cmd := exec.Command(compile, "-invalid-ddtest-flag")
	err = cmd.Run()
	native, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected native failure: %v", err)
	}
	if got := RunCoverTool(context.Background(), "missing-overlay", []string{compile, "-invalid-ddtest-flag"}, nil, &stdout, &stderr); got != native.ExitCode() {
		t.Fatalf("exit=%d, native=%d", got, native.ExitCode())
	}
}

func TestCoverCommandQuoting(t *testing.T) {
	for _, paths := range [][2]string{{"/path with spaces/ddtest", "/path/overlay.json"}, {`C:\Program Files\ddtest.exe`, `C:\build\overlay.json`}, {"/path/it's/ddtest", `/path/\"quoted\"/overlay.json`}} {
		command, err := coverToolCommand(paths[0], paths[1])
		if err != nil {
			t.Fatal(err)
		}
		got, err := splitFlags(command)
		want := []string{paths[0], "tool-overlay", "cover", paths[1]}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("quoted command=%q: %v, %v", command, got, err)
		}
	}
	if _, err := coverToolCommand(`path'with"both`, "overlay"); err == nil {
		t.Fatal("ambiguous quoting accepted")
	}
}

func TestCoverIdentityTracksOnlyTransformationInputs(t *testing.T) {
	a := Overlay{Replace: map[string]string{"source": "/temporary/one"}, CoverExclude: []string{"b.go", "a.go"}}
	b := Overlay{Replace: map[string]string{"source": "/temporary/two"}, CoverExclude: []string{"a.go", "b.go"}}
	if coverFingerprint(a) != coverFingerprint(b) {
		t.Fatal("temporary backing paths or exclusion ordering invalidated cache")
	}
	b.CoverExclude = []string{"a.go"}
	if coverFingerprint(a) == coverFingerprint(b) {
		t.Fatal("coverage exclusion change retained stale tool identity")
	}
	for _, version := range []string{"cover version go1.27.1\n", "cover version devel go1.28 buildID=action/content\n"} {
		tagged := appendCoverIdentity(version, "fixture")
		if !strings.Contains(tagged, "fixture") || !strings.HasPrefix(tagged, strings.TrimSpace(version)) {
			t.Fatalf("invalid tool identity: %q", tagged)
		}
	}
}
