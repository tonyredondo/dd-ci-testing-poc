package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A test that fails and then skips fails, as testing reports it, also without
// a retry or test management feature, which close the event themselves.
func TestMiniSkipAfterFailureReportsFailure(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "zz_skip_after_failure_test.go"), []byte(`package fixture_test

import "testing"

func TestFailThenSkip(t *testing.T) {
	t.Error("failure")
	t.Skip("reason")
}

func TestFailThenSkipNow(t *testing.T) {
	t.Error("failure")
	t.SkipNow()
}

func TestOnlySkip(t *testing.T) { t.Skip("reason") }
`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := execute(t, dir, compileMini(t, dir, driver), []string{"-test.run=^Test(FailThenSkip|FailThenSkipNow|OnlySkip)$"}, true, false)
	statuses := map[string]string{}
	for _, event := range got.wireEvents {
		content, _ := event["content"].(map[string]any)
		meta, _ := content["meta"].(map[string]any)
		if event["type"] == "test" {
			statuses[fmt.Sprint(meta["test.name"])] = fmt.Sprint(meta["test.status"], "/", meta["test.final_status"])
		}
	}
	want := map[string]string{"TestFailThenSkip": "fail/fail", "TestFailThenSkipNow": "fail/fail", "TestOnlySkip": "skip/skip"}
	if got.code != 1 || fmt.Sprint(statuses) != fmt.Sprint(want) {
		t.Fatalf("exit %d, statuses %v; want 1, %v\n%s", got.code, statuses, want, got.stderr)
	}
}
