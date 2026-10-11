package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ddto reads the GitHub Actions job ID once and hands it to each test process,
// which removes the variable before any client code runs.
func TestMiniGitHubJobIDHandoff(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	source := `package fixture_test

import (
	"os"
	"testing"
)

func TestHandoffHidden(t *testing.T) {
	if value, ok := os.LookupEnv("DDTO_GITHUB_JOB_ID"); ok {
		t.Fatalf("handoff visible to the client: %q", value)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "handoff_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), executableName("handoff.test"))
	out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false"), driver, "test", "--runtime=mini", "-c", "-o", bin, ".")
	if code != 0 {
		t.Fatalf("compile: %d %s %s", code, out, stderr)
	}
	github := []string{"GITHUB_SHA=0123456789abcdef0123456789abcdef01234567", "GITHUB_SERVER_URL=https://github.com", "GITHUB_REPOSITORY=tonyredondo/dd-ci-testing-poc", "GITHUB_RUN_ID=123", "GITHUB_RUN_ATTEMPT=1", "GITHUB_JOB=unit", "JOB_CHECK_RUN_ID="}
	for _, c := range []struct {
		handoff, jobID, url string
	}{
		{"DDTO_GITHUB_JOB_ID=4242", `"ci.job.id":"4242"`, `/actions/runs/123/job/4242"`},
		// The CLI found no ID: the job name and checks URL remain.
		{"DDTO_GITHUB_JOB_ID=", `"ci.job.id":"unit"`, `/commit/0123456789abcdef0123456789abcdef01234567/checks"`},
	} {
		capture := runCIWireCase(t, dir, bin, []string{"-test.run=^TestHandoffHidden$"}, append(github, c.handoff), false, false)
		payloads, err := json.Marshal(capture.payloads)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(payloads), c.jobID) || !strings.Contains(string(payloads), c.url) {
			t.Errorf("%s: job ID %s and URL %s missing from %s", c.handoff, c.jobID, c.url, payloads)
		}
	}
}
