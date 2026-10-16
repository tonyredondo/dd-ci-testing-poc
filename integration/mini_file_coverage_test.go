package integration

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A fixture whose tests reach different files: through the package's own
// code, another package of the module, a package that only tests import,
// and an embedded file that a test reads without running its package's code.
var fileCoverageFixture = map[string]string{
	"filecov.go": `package fixture

import "example.com/dd-ci-testing-fixture/lib"

func FileA() int { return lib.Lib() + 1 }

func FileB() int { return 2 }
`,
	"lib/lib.go":   "package lib\n\nfunc Lib() int { return 1 }\n",
	"lib/embed.go": "package lib\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar Data string\n",
	"lib/data.txt": "data\n",
	"kit/kit.go":   "package kit\n\nfunc Want(n int) int { return n }\n",
	// back imports the package under test, which has internal tests, so the
	// build with tests lists back only as the copy recompiled for them.
	"back/back.go":             "package back\n\nimport fixture \"example.com/dd-ci-testing-fixture\"\n\nfunc Back() int { return fixture.FileB() }\n",
	"source.go":                "package fixture\n\nfunc Source() int { return 4 }\n",
	"filecov_internal_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestFileInternal(t *testing.T) {\n\tif FileB() != 2 {\n\t\tt.Fatal(\"B\")\n\t}\n}\n",
	"filecov_test.go": `package fixture_test

import (
	_ "embed"
	"os"
	"testing"

	fixture "example.com/dd-ci-testing-fixture"
	"example.com/dd-ci-testing-fixture/back"
	"example.com/dd-ci-testing-fixture/kit"
	"example.com/dd-ci-testing-fixture/lib"
)

//go:embed source.go
var source string

func TestFileA(t *testing.T) {
	if fixture.FileA() != 2 {
		t.Fatal("A")
	}
}

func TestFileB(t *testing.T) {
	t.Parallel()
	if fixture.FileB() != kit.Want(2) {
		t.Fatal("B")
	}
}

func TestFileBack(t *testing.T) {
	if back.Back() != 2 {
		t.Fatal("back")
	}
}

func TestFileEmbed(t *testing.T) {
	if lib.Data != "data\n" {
		t.Fatal("embed")
	}
}

// An embedded Go file has the content on disk, not marks.
func TestFileEmbedSource(t *testing.T) {
	disk, err := os.ReadFile("source.go")
	if err != nil || source != string(disk) {
		t.Fatalf("embedded source.go differs from the file: %v\n%s", err, source)
	}
}
`,
}

func writeFileCoverageFixture(t *testing.T, dir string) {
	t.Helper()
	for name, content := range fileCoverageFixture {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

// fileCoverageRows returns each test's covered files by base name, and
// whether any row carries a line bitmap.
func fileCoverageRows(t *testing.T, c *miniWireCapture) (map[string][]string, bool) {
	t.Helper()
	files, bitmaps := map[string][]string{}, false
	for _, row := range normalizedMiniCoverage(t, c) {
		fields := strings.Fields(row)
		files[fields[0]] = append(files[fields[0]], fields[1])
		// A row without a bitmap renders it as %!x(<nil>).
		bitmaps = bitmaps || len(fields) > 2 && !strings.HasPrefix(fields[2], "%!")
	}
	for test := range files {
		slices.Sort(files[test])
		files[test] = slices.Compact(files[test])
	}
	return files, bitmaps
}

// With --experimental-file-coverage, each test's coverage comes from file
// marks: the files its code ran, in any package of the module, including a
// package that only tests import, plus the files every test reports (the
// embedded files, a Go file among them, and the file whose only code is a
// declaration). Every production file that Go's counters report is there
// too. Rows carry no line bitmaps, and go test prints no coverage lines.
func TestMiniFileCoverage(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeFileCoverageFixture(t, dir)
	env := testEnv("DD_CIVISIBILITY_ENABLED=false", "DD_TRACE_DEBUG=true")
	build := func(name string, flags ...string) (string, string) {
		t.Helper()
		bin := filepath.Join(t.TempDir(), executableName(name))
		args := append(append([]string{"test", "--runtime=mini"}, flags...), "-c", "-o", bin, ".")
		out, stderr, code := command(t, dir, env, driver, args...)
		if code != 0 {
			t.Fatal(out, stderr)
		}
		return bin, stderr
	}
	run := func(bin string) (map[string][]string, bool, execution) {
		t.Helper()
		got, exec := runParityCase(t, dir, bin, parityCase{Args: []string{"-test.v", "-test.run=^TestFile"}, Policy: policySettings{Coverage: true}, Coverage: true})
		if exec.code != 0 {
			t.Fatalf("exit %d:\n%s\n%s", exec.code, exec.out, exec.stderr)
		}
		files, bitmaps := fileCoverageRows(t, &got.miniWireCapture)
		return files, bitmaps, exec
	}

	marksBin, buildLog := build("marks.test", "--experimental-file-coverage")
	if !strings.Contains(buildLog, "file coverage installed=true") {
		t.Fatalf("build log lacks the installed file coverage:\n%s", buildLog)
	}
	marks, bitmaps, exec := run(marksBin)
	if bitmaps {
		t.Errorf("file coverage rows carry bitmaps: %v", marks)
	}
	if strings.Contains(exec.out, "coverage: ") {
		t.Errorf("test process printed coverage lines:\n%s", exec.out)
	}
	always := []string{"data.txt", "embed.go", "source.go"}
	want := map[string][]string{
		"TestFileA":           {"filecov.go", "filecov_test.go", "lib.go"},
		"TestFileB":           {"filecov.go", "filecov_test.go", "kit.go"},
		"TestFileBack":        {"back.go", "filecov.go", "filecov_test.go"},
		"TestFileInternal":    {"filecov.go", "filecov_internal_test.go"},
		"TestFileEmbed":       {"filecov_test.go"},
		"TestFileEmbedSource": {"filecov_test.go"},
	}
	for test, files := range want {
		files = append(files, always...)
		slices.Sort(files)
		if !slices.Equal(marks[test], files) {
			t.Errorf("%s: %v, want %v", test, marks[test], files)
		}
	}

	countersBin, _ := build("counters.test", "-covermode=atomic", "-coverpkg=./...")
	counters, _, _ := run(countersBin)
	if len(counters["TestFileA"]) == 0 || len(counters["TestFileB"]) == 0 {
		t.Fatalf("Go's counters gave no coverage to compare: %v", counters)
	}
	for test, files := range counters {
		for _, file := range files {
			if !slices.Contains(marks[test], file) {
				t.Errorf("%s: Go's counters report %s, file marks do not: %v", test, file, marks[test])
			}
		}
	}
}

// The flag keeps Go's counters, and says why, when the build cannot take
// marks: with Go's coverage flags, also through GOFLAGS, and under -race.
func TestMiniFileCoverageRefusals(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeFileCoverageFixture(t, dir)
	for _, tc := range []struct {
		name   string
		env    []string
		flags  []string
		reason string
	}{
		{"cover", nil, []string{"-cover"}, "Go's coverage flags are set"},
		{"goflags cover", []string{"GOFLAGS=-buildvcs=false -covermode=atomic"}, nil, "Go's coverage flags are set"},
		{"race", nil, []string{"-race"}, "-race"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), executableName("refused.test"))
			args := append(append([]string{"test", "--runtime=mini", "--experimental-file-coverage"}, tc.flags...), "-c", "-o", bin, ".")
			env := append(testEnv("DD_CIVISIBILITY_ENABLED=false", "DD_TRACE_DEBUG=true"), tc.env...)
			out, stderr, code := command(t, dir, env, driver, args...)
			if code != 0 {
				t.Fatal(out, stderr)
			}
			if want := "--experimental-file-coverage: " + tc.reason + "; per-test coverage keeps Go's coverage counters"; !strings.Contains(stderr, want) {
				t.Fatalf("stderr lacks %q:\n%s", want, stderr)
			}
			if !strings.Contains(stderr, "file coverage installed=false") {
				t.Fatalf("build log lacks the refusal:\n%s", stderr)
			}
		})
	}
}

// A test binary that can load plugins keeps Go's counters, as the in-memory
// coverage hook does: a plugin shares packages with the binary, and one built
// without the marks would not load. Here only a test imports the package that
// uses plugin.
func TestMiniFileCoverageRefusesPlugins(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeFileCoverageFixture(t, dir)
	if err := os.Mkdir(filepath.Join(dir, "loader"), 0700); err != nil {
		t.Fatal(err)
	}
	writeBuildFixture(t, dir, map[string]string{
		"loader/loader.go": "package loader\n\nimport \"plugin\"\n\nfunc Open(path string) error {\n\t_, err := plugin.Open(path)\n\treturn err\n}\n",
		"loader_test.go":   "package fixture_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/dd-ci-testing-fixture/loader\"\n)\n\nfunc TestLoader(t *testing.T) { _ = loader.Open }\n",
	})
	bin := filepath.Join(t.TempDir(), executableName("plugin.test"))
	out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false", "DD_TRACE_DEBUG=true"), driver,
		"test", "--runtime=mini", "--experimental-file-coverage", "-c", "-o", bin, ".")
	if code != 0 {
		t.Fatal(out, stderr)
	}
	if want := `file coverage installed=false reason="plugin"`; !strings.Contains(stderr, want) {
		t.Fatalf("build log lacks %q:\n%s", want, stderr)
	}
}

// File marks keep go test's output and its test cache: a second run is
// cached, and a run with marks does not reuse the result of a run without.
func TestMiniFileCoverageTestCache(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeFileCoverageFixture(t, dir)
	cache := t.TempDir()
	env := testEnv("DD_CIVISIBILITY_ENABLED=false", "GOCACHE="+cache)
	test := func(flags ...string) string {
		t.Helper()
		args := append(append([]string{"test", "--runtime=mini"}, flags...), "-run=^TestFile", ".")
		out, stderr, code := command(t, dir, env, driver, args...)
		if code != 0 {
			t.Fatal(out, stderr)
		}
		return out
	}
	if out := test(); strings.Contains(out, "(cached)") {
		t.Fatalf("first run cached:\n%s", out)
	}
	if out := test("--experimental-file-coverage"); strings.Contains(out, "(cached)") || strings.Contains(out, "coverage:") {
		t.Fatalf("first run with marks:\n%s", out)
	}
	if out := test("--experimental-file-coverage"); !strings.Contains(out, "(cached)") {
		t.Fatalf("second run with marks not cached:\n%s", out)
	}
	out, stderr, code := command(t, dir, env, driver, "test", "--runtime=mini", "--experimental-file-coverage", "-json", "-run=^TestFile", ".")
	if code != 0 || !strings.Contains(out, `"Action":"pass"`) {
		t.Fatalf("-json: exit %d\n%s\n%s", code, out, stderr)
	}
}
