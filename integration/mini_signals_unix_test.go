//go:build unix

package integration

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Signals keep go test's behavior. A test that handles SIGTERM itself keeps
// its process, so the tests after it run and report; Mini's channel does not
// receive that SIGTERM, even though the test stops its own channel at once. A
// SIGTERM that nothing handles still ends the process with that signal, as
// under go test, after Mini has sent its events.
func TestMiniSignalsKeepGoTestBehavior(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeBuildFixture(t, dir, map[string]string{"sample_test.go": `package fixture_test

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestHandlesTERM(t *testing.T) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-c:
	case <-time.After(time.Minute):
		t.Fatal("SIGTERM never arrived")
	}
	signal.Stop(c)
}

func TestAfter(t *testing.T) {}

func TestTERM(t *testing.T) {
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Minute)
	t.Fatal("SIGTERM did not end the process")
}
`})
	for _, tc := range []struct {
		name, run string
		code      int
		output    string
		tests     []string
	}{
		{"handled", "^(TestHandlesTERM|TestAfter)$", 0, "--- PASS: TestAfter", []string{"TestAfter", "TestHandlesTERM"}},
		{"unhandled", "^(TestAfter|TestTERM)$", 1, "signal: terminated", []string{"TestAfter"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receiver := &capture{}
			server := newServer(http.HandlerFunc(receiver.handler))
			defer server.Close()
			env := testEnv("DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL, "DD_TRACE_AGENT_URL="+server.URL, "DD_API_KEY=poc-not-a-real-key")
			out, stderr, code := command(t, dir, env, driver, "test", "--runtime=mini", "-count=1", "-v", "-run="+tc.run, ".")
			if code != tc.code || !strings.Contains(out, tc.output) {
				t.Fatalf("exit %d, want %d and %q\n%s\n%s", code, tc.code, tc.output, out, stderr)
			}
			receiver.mu.Lock()
			defer receiver.mu.Unlock()
			var tests []string
			sessions := 0
			for _, event := range receiver.events {
				content, _ := event["content"].(map[string]any)
				meta, _ := content["meta"].(map[string]any)
				switch event["type"] {
				case "test":
					tests = append(tests, fmt.Sprint(meta["test.name"]))
				case "test_session_end":
					sessions++
				}
			}
			sort.Strings(tests)
			if len(receiver.failures) != 0 || !reflect.DeepEqual(tests, tc.tests) || sessions != 1 {
				t.Fatalf("tests %v, sessions %d, failures %v\n%s\n%s", tests, sessions, receiver.failures, out, stderr)
			}
		})
	}
}
