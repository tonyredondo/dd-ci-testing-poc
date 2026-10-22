package integration

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// Without -v, go test prints a failing test's output. A test that still
// fails after its automatic retries keeps that diagnostic. Process retries run
// after testing reported the package, which the README describes.
func TestMiniRetriedFailureKeepsItsOutput(t *testing.T) {
	driver := sharedDriver(t, "..")
	dir := t.TempDir()
	writeBuildFixture(t, dir, map[string]string{
		"go.mod":        "module example.com/retryoutput\ngo 1.25\n",
		"retry_test.go": "package retryoutput\n\nimport \"testing\"\n\nfunc TestDiagnostic(t *testing.T) { t.Fatal(\"DIAGNOSTIC_SENTINEL\") }\n",
	})
	bin := filepath.Join(t.TempDir(), executableName("fixture.test"))
	if out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false", "GOWORK=off"), driver, "test", "-c", "-o", bin, "."); code != 0 {
		t.Fatalf("compile: %s\n%s", out, stderr)
	}
	for _, mode := range []string{"in_process"} {
		receiver := &capture{retry: true}
		server := newServer(http.HandlerFunc(receiver.handler))
		env := testEnv("DD_CIVISIBILITY_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL, "DD_TRACE_AGENT_URL="+server.URL, "DD_API_KEY=poc-not-a-real-key", "DD_CIVISIBILITY_FLAKY_RETRY_ENABLED=true", "DD_CIVISIBILITY_FLAKY_RETRY_COUNT=1", "DD_CIVISIBILITY_RETRY_EXECUTION_MODE="+mode)
		out, stderr, code := command(t, dir, env, bin, "-test.run=^TestDiagnostic$")
		server.Close()
		if code != 1 || !strings.Contains(out, "DIAGNOSTIC_SENTINEL") {
			t.Errorf("%s retries: exit %d; want 1 with the failure's output\n%s\n%s", mode, code, out, stderr)
		}
	}
}
