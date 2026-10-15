package integration

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// os.Exit and log.Fatal while tests run end the process with go test's
// result, and Mini still sends the events of the tests that finished and the
// failed session: from the test body, from a cleanup, from another
// goroutine, and with deferred delivery. Mini closes a test when its body
// returns, before testing runs its cleanups, so a test whose cleanup exits
// reports its body's result.
func TestMiniForcedExitKeepsEvents(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeBuildFixture(t, dir, map[string]string{"sample_test.go": `package fixture_test

import (
	"log"
	"os"
	"testing"
	"time"
)

func TestFirst(t *testing.T) {}

func TestExit(t *testing.T) {
	switch os.Getenv("FIXTURE_EXIT") {
	case "body":
		os.Exit(3)
	case "fatal":
		log.Fatal("fixture fatal")
	case "cleanup":
		t.Cleanup(func() { os.Exit(4) })
	case "goroutine":
		go os.Exit(5)
		time.Sleep(time.Minute)
		t.Fatal("os.Exit did not end the process")
	}
}
`})
	first := []string{"TestFirst pass"}
	for _, tc := range []struct {
		name, exit string
		deferred   bool
		tests      []string
	}{
		{"body", "body", false, first},
		{"fatal", "fatal", false, first},
		{"cleanup", "cleanup", false, []string{"TestFirst pass", "TestExit pass"}},
		{"goroutine", "goroutine", false, first},
		{"deferred", "body", true, first},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receiver := &capture{}
			server := newServer(http.HandlerFunc(receiver.handler))
			defer server.Close()
			env := testEnv("DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL, "DD_TRACE_AGENT_URL="+server.URL, "DD_API_KEY=poc-not-a-real-key",
				"FIXTURE_EXIT="+tc.exit, fmt.Sprintf("DD_CIVISIBILITY_DEFERRED_DELIVERY=%t", tc.deferred))
			out, stderr, code := command(t, dir, env, driver, "test", "--runtime=mini", "-count=1", "-v", ".")
			if code != 1 || !strings.Contains(out, "--- PASS: TestFirst") || strings.Contains(out+stderr, "exit hook invoked") || strings.Contains(out+stderr, "fatal error") {
				t.Fatalf("exit %d\n%s\n%s", code, out, stderr)
			}
			receiver.mu.Lock()
			defer receiver.mu.Unlock()
			var tests, sessions []string
			for _, event := range receiver.events {
				content, _ := event["content"].(map[string]any)
				meta, _ := content["meta"].(map[string]any)
				switch event["type"] {
				case "test":
					tests = append(tests, fmt.Sprint(meta["test.name"])+" "+fmt.Sprint(meta["test.status"]))
				case "test_session_end":
					sessions = append(sessions, fmt.Sprint(meta["test.status"]))
				}
			}
			if len(receiver.failures) != 0 || !reflect.DeepEqual(tests, tc.tests) || !reflect.DeepEqual(sessions, []string{"fail"}) {
				t.Fatalf("tests %v, sessions %v, failures %v\n%s\n%s", tests, sessions, receiver.failures, out, stderr)
			}
		})
	}
}
