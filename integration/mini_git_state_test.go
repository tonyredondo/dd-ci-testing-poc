package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// gitStateFixture prepares a Mini fixture in a two-commit Git repository with
// a remote and returns its directory, the CLI driver and a compiled binary.
func gitStateFixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir, driver := prepareMiniFixture(t)
	prepareParityGit(t, dir)
	if out, stderr, code := command(t, dir, testEnv(), "git", "remote", "add", "origin", "https://example.invalid/acme/fixture.git"); code != 0 {
		t.Fatalf("remote: %s %s", out, stderr)
	}
	bin := filepath.Join(t.TempDir(), executableName("git-state.test"))
	if out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false"), driver, "test", "--runtime=mini", "-c", "-o", bin, "."); code != 0 {
		t.Fatalf("compile: %d %s %s", code, out, stderr)
	}
	return dir, driver, bin
}

// runGitWorker runs ddto's Git worker in dir and returns its state directory.
// input is what the CLI would write to it; empty input stops the worker after
// the Git record.
func runGitWorker(t *testing.T, dir, driver, input string, env ...string) string {
	t.Helper()
	state := t.TempDir()
	cmd := exec.Command(driver, "ci-git-worker", state)
	cmd.Dir = dir
	cmd.Env = append(testEnv(env...), "PWD="+dir)
	cmd.Stdin = strings.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worker: %v %s", err, out)
	}
	return state
}

// gitTags collects the Git and workspace tags of every payload.
func gitTags(t *testing.T, capture *miniWireCapture) map[string][]string {
	t.Helper()
	values := map[string]map[string]bool{}
	var visit func(any)
	visit = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			for key, value := range v {
				if text, ok := value.(string); ok && (strings.HasPrefix(key, "git.") || key == "ci.workspace_path") {
					if values[key] == nil {
						values[key] = map[string]bool{}
					}
					values[key][text] = true
				}
				visit(value)
			}
		case []any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	for _, payload := range capture.payloads {
		visit(payload)
	}
	tags := map[string][]string{}
	for key, set := range values {
		for value := range set {
			tags[key] = append(tags[key], value)
		}
		sort.Strings(tags[key])
	}
	if len(tags["git.commit.sha"]) != 1 || tags["git.repository_url"] == nil || tags["git.commit.message"] == nil {
		t.Fatalf("missing Git tags: %v", tags)
	}
	return tags
}

// gitCommands returns the Git commands that GIT_TRACE recorded.
func gitCommands(t *testing.T, trace string) string {
	t.Helper()
	data, err := os.ReadFile(trace)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// ddto reads a checkout's Git metadata once and hands it to each test process
// of that checkout, which then reports the same tags without running Git.
func TestMiniGitStateHandoff(t *testing.T) {
	dir, driver, bin := gitStateFixture(t)
	args := []string{"-test.run=^TestPass$"}
	trace := filepath.Join(t.TempDir(), "git-trace.log")
	// The tags must come from Git, not from testEnv's overrides.
	fromGit := []string{"DD_GIT_REPOSITORY_URL=", "DD_GIT_COMMIT_SHA="}
	env := func(extra ...string) []string { return append(append([]string(nil), fromGit...), extra...) }

	want := gitTags(t, runCIWireCase(t, dir, bin, args, env("GIT_TRACE="+trace), false, false))
	if commands := gitCommands(t, trace); !strings.Contains(commands, "rev-parse --show-toplevel") {
		t.Fatalf("baseline ran no Git metadata command:\n%s", commands)
	}

	state := runGitWorker(t, dir, driver, "", fromGit...)
	handedTrace := filepath.Join(t.TempDir(), "git-trace.log")
	got := gitTags(t, runCIWireCase(t, dir, bin, args, env("GIT_TRACE="+handedTrace, "DDTO_GIT_STATE="+state), false, false))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("handed Git tags differ:\n got %v\nwant %v", got, want)
	}
	commands := gitCommands(t, handedTrace)
	for _, metadata := range []string{"rev-parse --show-toplevel", "ls-remote --get-url", "rev-parse --abbrev-ref HEAD", "log -1"} {
		if strings.Contains(commands, metadata) {
			t.Errorf("test process ran git %s with handed state:\n%s", metadata, commands)
		}
	}

	// A state from another checkout is ignored.
	other := t.TempDir()
	for _, gitArgs := range [][]string{{"init", "--initial-branch=main"}, {"-c", "user.name=Other", "-c", "user.email=other@example.invalid", "commit", "--allow-empty", "-m", "Other"}} {
		if out, stderr, code := command(t, other, testEnv(), "git", gitArgs...); code != 0 {
			t.Fatalf("other repository: %s %s", out, stderr)
		}
	}
	otherState := runGitWorker(t, other, driver, "", fromGit...)
	otherTrace := filepath.Join(t.TempDir(), "git-trace.log")
	if got := gitTags(t, runCIWireCase(t, dir, bin, args, env("GIT_TRACE="+otherTrace, "DDTO_GIT_STATE="+otherState), false, false)); !reflect.DeepEqual(got, want) {
		t.Fatalf("another checkout's state changed the tags:\n got %v\nwant %v", got, want)
	}
	if commands := gitCommands(t, otherTrace); !strings.Contains(commands, "rev-parse --show-toplevel") {
		t.Fatalf("another checkout's state replaced Git:\n%s", commands)
	}
}

// A ddto test run uploads the repository once, from its Git worker, instead
// of once per test process.
func TestMiniGitStateUploadsOnce(t *testing.T) {
	dir, driver, _ := gitStateFixture(t)
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "sub_test.go"), []byte("package sub\n\nimport \"testing\"\n\nfunc TestSub(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		requests[path.Base(r.URL.Path)]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/setting") {
			fmt.Fprint(w, `{"data":{"id":"poc","type":"ci_app_test_service_libraries_settings","attributes":{"require_git":false,"impacted_tests_enabled":false,"early_flake_detection":{"enabled":false},"test_management":{"enabled":false}}}}`)
			return
		}
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	env := testEnv("DD_CIVISIBILITY_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL,
		"DD_API_KEY=fixture", "DD_CIVISIBILITY_GIT_UPLOAD_ENABLED=true", "DD_GIT_REPOSITORY_URL=", "DD_GIT_COMMIT_SHA=")
	if out, stderr, code := command(t, dir, env, driver, "test", "--runtime=mini", "-count=1", "-run=^Test(Pass|Sub)$", ".", "./sub"); code != 0 {
		t.Fatalf("ddto test: %d %s %s", code, out, stderr)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests["search_commits"] != 1 || requests["packfile"] != 1 {
		t.Fatalf("two test processes: %v", requests)
	}
}
