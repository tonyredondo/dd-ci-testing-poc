package integration

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// This is an explicit coverage gap, not a passing parity case. The SDK with
// its full testing YAML registers Testify methods. The POC's testing-only
// transformation currently does not modify the third-party suite package.
func TestCIVisibilityTestifyCoverageGap(t *testing.T) {
	reference := os.Getenv("ORCHESTRION_BIN")
	if reference == "" {
		t.Skip("Testify gap requires frozen Orchestrion reference")
	}
	reference, err := filepath.Abs(reference)
	if err != nil {
		t.Fatal(err)
	}
	dir, driver := prepareFixture(t, true)
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"mod", "edit", "-require=github.com/tonyredondo/dd-ci-testing-poc@v0.0.0"}, {"mod", "edit", "-replace=github.com/tonyredondo/dd-ci-testing-poc=" + root}} {
		out, stderr, code := command(t, dir, testEnv(), "go", args...)
		if code != 0 {
			t.Fatalf("prepare Mini: %s %s", out, stderr)
		}
	}
	file := filepath.Join(dir, "sample_test.go")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, neutralMiniFixture(body), 0600); err != nil {
		t.Fatal(err)
	}
	source := `package fixture_test

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type ParitySuite struct{ suite.Suite }

func (s *ParitySuite) TestPass()   { s.T().Log("suite pass") }
func (s *ParitySuite) TestSkip()   { s.T().Skip("suite skip") }
func TestParitySuite(t *testing.T) { suite.Run(t, new(ParitySuite)) }
`
	if err = os.WriteFile(filepath.Join(dir, "testify_cases_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	bins := compileMiniPair(t, dir, driver, "-mod=mod")
	baseline := filepath.Join(t.TempDir(), executableName("fixture.test"))
	out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false"), "go", "test", "-mod=mod", "-toolexec="+reference+" toolexec", "-c", "-o", baseline, ".")
	if code != 0 {
		t.Fatalf("reference compile: %s %s", out, stderr)
	}
	tc := parityCase{Args: []string{"-test.run=^TestParitySuite$"}}
	sdk, sdkResult := runParityCase(t, dir, bins[0], tc)
	mini, miniResult := runParityCase(t, dir, bins[1], tc)
	full, fullResult := runParityCase(t, dir, baseline, tc)
	if sdkResult.code != 0 || miniResult.code != 0 || fullResult.code != 0 {
		t.Fatalf("Testify execution failed: %s %s %s", sdkResult.stderr, miniResult.stderr, fullResult.stderr)
	}
	sdkCounts, err := countCIEvents(sdk.events)
	if err != nil {
		t.Fatal(err)
	}
	miniCounts, err := countCIEvents(mini.events)
	if err != nil {
		t.Fatal(err)
	}
	fullCounts, err := countCIEvents(full.events)
	if err != nil {
		t.Fatal(err)
	}
	assertMiniCIAttributes(t, sdk.events, mini.events)
	fullRows, miniRows := ciWireEvents(full.events), ciWireEvents(mini.events)
	if reflect.DeepEqual(fullRows, miniRows) {
		t.Fatal("Testify coverage gap changed; review and promote this fixture to strict parity")
	}
	// Freeze the observed gap so another lost/duplicated hierarchy cannot be
	// accepted merely because the implementations still differ.
	if fullCounts != (eventCounts{1, 1, 2, 3, 0}) || sdkCounts != (eventCounts{1, 2, 3, 3, 0}) || miniCounts != sdkCounts {
		t.Fatalf("Testify gap changed: SDK %+v POC SDK %+v Mini %+v", fullCounts, sdkCounts, miniCounts)
	}
	for _, receiver := range []*parityReceiver{full, sdk, mini} {
		if err := validateEventGraph(receiver.events); err != nil {
			t.Fatal(err)
		}
	}
	writeParityEvidence(t, "testify", map[string]any{"timing": parityTiming{binaryTimingScope, fullResult.wall.Nanoseconds(), miniResult.wall.Nanoseconds()}, "poc_sdk_wall_ns": sdkResult.wall.Nanoseconds(), "status": "gap", "reason": "missing testify.suite.Run advice; method suite/source metadata differ", "sdk_with_orchestrion": fullCounts, "sdk_with_poc": sdkCounts, "mini": miniCounts})
	t.Logf("Known Testify gap: full SDK %+v, POC SDK %+v, Mini %+v", fullCounts, sdkCounts, miniCounts)
}
