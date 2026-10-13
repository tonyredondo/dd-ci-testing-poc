package runner

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A -coverpkg pattern that could select a standard package or one of the
// Mini module keeps text profiles; a pattern that only selects client
// packages gets the in-memory hook.
func TestCoverPatternReachesCollector(t *testing.T) {
	root := t.TempDir()
	client, mini, goroot := filepath.Join(root, "work", "client"), filepath.Join(root, "work", "mini"), filepath.Join(root, "go")
	for _, tc := range []struct {
		pattern  string
		miniMain bool
		reaches  bool
	}{
		{"all", false, true}, {"std", false, true}, {"cmd", false, true},
		{"work", false, false}, {"work", true, true}, {"tool", true, true},
		{"internal/...", false, true}, {".../coverage/...", false, true}, {"...", false, true}, {"testing", false, true},
		{"github.com/...", false, true}, {"github.com/tonyredondo/...", false, true}, {"github.com/tonyredondo/dd-ci-testing-poc", false, true},
		{"github.com/tonyredondo/dd-ci-testing-poc/internal/...", false, true}, {"github.com/tonyredondo/dd-ci-testing-poc...", false, true},
		{"example.com/app/...", false, false}, {"example.com/app", false, false}, {"github.com/other/...", false, false},
		{"github.com/tonyredondo/dd-ci-testing-pocket/...", false, false}, {"github.com/tonyredondo/dd-ci-testing-p...", false, true},
		{"./...", false, false}, {".", false, false}, {"../...", false, true}, {"../mini/...", false, true}, {"../other/...", false, false},
		{"../../...", false, true}, {filepath.Join(goroot, "src") + "/...", false, true}, {filepath.Join(root, "work") + "/...", false, true},
		{filepath.Join(root, "elsewhere") + "/...", false, false},
	} {
		if got := coverPatternReachesCollector(tc.pattern, client, mini, tc.miniMain, []string{goroot}); got != tc.reaches {
			t.Errorf("%q (Mini main %t) = %t, want %t", tc.pattern, tc.miniMain, got, tc.reaches)
		}
	}
}

// coverageHookRefusal accepts a client-only selection and refuses plugins,
// standard or Mini packages under test, unclear selections and runtimes
// without the registration.
func TestCoverageHookRefusal(t *testing.T) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	goroot := strings.TrimSpace(string(out))
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	miniModuleInfo := &struct {
		Path, Version, Dir string
		Main               bool
		Replace            *struct{ Path, Dir, Version string }
	}{Path: miniModule, Dir: repo}
	clientModule := &struct {
		Path, Version, Dir string
		Main               bool
		Replace            *struct{ Path, Dir, Version string }
	}{Path: "example.com/app", Dir: "/client", Main: true}
	base := func() []goPackage {
		return []goPackage{
			{ImportPath: "example.com/app", Dir: "/client", Module: clientModule, TestGoFiles: []string{"app_test.go"}, Imports: []string{"fmt"}},
			{ImportPath: "testing", Standard: true, TestGoFiles: []string{"testing_test.go"}},
			{ImportPath: miniPackage, Module: miniModuleInfo, DepOnly: true},
			{ImportPath: miniCoveragePackage, Dir: filepath.Join(repo, "internal", "thirdparty", "dd-trace-go", "civisibility", "integrations", "gotesting", "coverage"), Module: miniModuleInfo, DepOnly: true},
		}
	}
	refusal := func(opts options, listed []goPackage, testPlugin bool) string {
		return coverageHookRefusal("/client", opts, miniPackage, []string{goroot}, listed, testPlugin)
	}
	if got := refusal(options{coverage: true}, base(), false); got != "" {
		t.Fatalf("client package under test refused: %s", got)
	}
	if got := refusal(options{coverage: true, coverPatterns: []string{"./..."}}, base(), false); got != "" {
		t.Fatalf("client selection refused: %s", got)
	}
	cases := map[string]func() (options, []goPackage, bool){
		"plugin in graph": func() (options, []goPackage, bool) {
			return options{coverage: true}, append(base(), goPackage{ImportPath: "plugin", Standard: true, DepOnly: true}), false
		},
		"plugin imported by tests": func() (options, []goPackage, bool) {
			listed := base()
			listed[0].XTestImports = []string{"plugin"}
			return options{coverage: true}, listed, false
		},
		"plugin behind a test import": func() (options, []goPackage, bool) { return options{coverage: true}, base(), true },
		"standard selection": func() (options, []goPackage, bool) {
			return options{coverage: true, coverPatterns: []string{"./...", "testing"}}, base(), false
		},
		"Mini package under test": func() (options, []goPackage, bool) {
			listed := base()
			listed[0].Module = miniModuleInfo
			return options{coverage: true}, listed, false
		},
		"runtime without coverage package": func() (options, []goPackage, bool) {
			return options{coverage: true}, base()[:3], false
		},
		"runtime without registration": func() (options, []goPackage, bool) {
			listed := base()
			listed[3].Dir = filepath.Join(repo, "internal", "runner")
			return options{coverage: true}, listed, false
		},
	}
	for name, setup := range cases {
		opts, listed, testPlugin := setup()
		if got := refusal(opts, listed, testPlugin); got == "" {
			t.Errorf("%s: accepted", name)
		}
	}
}
