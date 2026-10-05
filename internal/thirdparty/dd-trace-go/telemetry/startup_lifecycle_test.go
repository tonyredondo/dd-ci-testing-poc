package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/cidelivery"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry/internal"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry/internal/transport"
)

type startupEnvelope struct {
	Type    string          `json:"request_type"`
	Time    int64           `json:"tracer_time"`
	Seq     int             `json:"seq_id"`
	Payload json.RawMessage `json:"payload"`
}

// The receiver runs in this process. Explicitly synchronize its clock reads
// with the mutation; TCP delivery alone is not a race-detector synchronization.
type startupClockListener struct {
	net.Listener
	ready <-chan struct{}
}

func (l startupClockListener) Accept() (net.Conn, error) {
	<-l.ready
	return l.Listener.Accept()
}

// Each subprocess owns a fresh global client, enabled flag and delivery queue.
func TestStartupTelemetryWaitsForIdleGroup(t *testing.T) {
	const helperEnv = "DDTEST_TELEMETRY_STARTUP_HELPER"
	if os.Getenv(helperEnv) == "" {
		for _, mode := range []string{"false", "true"} {
			for _, scenario := range []string{"group", "no-tests", "retry", "concurrent-close", "clock", "panic"} {
				t.Run("deferred="+mode+"/"+scenario, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStartupTelemetryWaitsForIdleGroup$", "-test.count=1")
					cmd.Env = append(os.Environ(), helperEnv+"="+scenario, "DD_INSTRUMENTATION_TELEMETRY_ENABLED=true", "DD_CIVISIBILITY_DEFERRED_DELIVERY="+mode)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("startup lifecycle: %v\n%s", err, out)
					}
				})
			}
		}
		return
	}
	scenario := os.Getenv(helperEnv)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var mu sync.Mutex
	var bodies []startupEnvelope
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body startupEnvelope
		if err := json.Unmarshal(data, &body); err != nil {
			t.Errorf("telemetry body: %v", err)
		}
		mu.Lock()
		bodies = append(bodies, body)
		n := len(bodies)
		mu.Unlock()
		if n == 1 {
			close(entered)
			<-release
		}
		if scenario == "retry" && n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	clockReady := make(chan struct{})
	var clockOnce sync.Once
	releaseClock := func() { clockOnce.Do(func() { close(clockReady) }) }
	if scenario == "clock" {
		server.Listener = startupClockListener{Listener: server.Listener, ready: clockReady}
	}
	server.Start()
	defer server.Close()
	defer unblock()
	defer releaseClock()
	httpClient := server.Client()
	httpClient.Timeout = 2 * time.Second
	interval := time.Hour
	if scenario == "clock" {
		interval = time.Millisecond
	}
	next, err := NewClient("startup-test", "", "", ClientConfig{
		AgentURL: server.URL, AgentlessURL: server.URL, APIKey: "startup-test-placeholder",
		HTTPClient: httpClient, FlushInterval: internal.Range[time.Duration]{Min: interval, Max: interval},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Repeated Close is not the client's contract; StopApp owns client shutdown.
	var panicCalls int
	if scenario == "panic" {
		next.AddFlushTicker(func(Client) { panicCalls++; panic("startup callback") })
	}
	next.RegisterAppConfig("startup-config", "initial", OriginEnvVar)
	before := time.Now().Unix()
	started := make(chan struct{})
	go func() { StartApp(next); close(started) }()
	startupTimeout := time.NewTimer(time.Second)
	select {
	case <-started:
	case <-startupTimeout.C:
		releaseClock()
		unblock()
		<-started
		StopApp()
		t.Fatal("StartApp waited for HTTP before any test ran")
	}
	startupTimeout.Stop()
	if scenario == "panic" {
		cidelivery.Shutdown()
		if !Disabled() || panicCalls != 1 {
			t.Fatalf("failed startup was queued again: disabled=%t calls=%d", Disabled(), panicCalls)
		}
		StopApp()
		if GlobalClient() != nil {
			t.Fatal("failed startup remained installed after close")
		}
		select {
		case <-entered:
			t.Fatal("failed startup attempted HTTP")
		default:
		}
		return
	}
	after := time.Now().Unix()
	next.RegisterAppConfig("startup-config", "changed", OriginEnvVar)
	select {
	case <-entered:
		t.Fatal("startup HTTP ran before a test group or session close")
	default:
	}

	if scenario == "no-tests" {
		unblock()
		StopApp()
	} else {
		first, second := cidelivery.Begin(), cidelivery.Begin()
		if scenario == "clock" {
			func() {
				previous := time.Local
				defer func() { time.Local = previous }()
				zones := []*time.Location{time.FixedZone("A", 3600), time.FixedZone("B", -3600)}
				for i := range 1000000 {
					time.Local = zones[i%2]
					if i%64 == 0 {
						runtime.Gosched()
					}
				}
			}()
			releaseClock()
		}
		// A periodic tick must not bypass the queued startup delivery.
		next.Flush()
		select {
		case <-entered:
			t.Fatal("startup HTTP overlapped the first admitted test group")
		default:
		}
		first()
		select {
		case <-entered:
			t.Fatal("startup HTTP began before the parallel group finished")
		default:
		}
		if scenario == "group" {
			time.Sleep(1100 * time.Millisecond)
			select {
			case <-entered:
				t.Fatal("startup HTTP ran while the parallel group remained active")
			default:
			}
		}
		finished := make(chan struct{})
		go func() { second(); close(finished) }()
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("idle group did not send startup")
		}
		closing := make(chan struct{})
		if scenario == "concurrent-close" {
			go func() { StopApp(); close(closing) }()
			select {
			case <-closing:
				t.Fatal("StopApp returned during startup HTTP")
			case <-time.After(10 * time.Millisecond):
			}
		}
		admitted := make(chan func(), 1)
		go func() { admitted <- cidelivery.Begin() }()
		select {
		case <-admitted:
			t.Fatal("the next test overlapped startup HTTP")
		case <-time.After(10 * time.Millisecond):
		}
		unblock()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Fatal("idle startup did not finish")
		}
		(<-admitted)()
		if scenario == "concurrent-close" {
			<-closing
		} else {
			StopApp()
		}
	}
	if GlobalClient() != nil {
		t.Fatal("session close left the client installed")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) < 2 || bodies[0].Type != "app-started" {
		t.Fatalf("startup missing or out of order: %+v", bodies)
	}
	var startup transport.AppStarted
	if err := json.Unmarshal(bodies[0].Payload, &startup); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, config := range startup.Configuration {
		if config.Name == "startup-config" {
			found = true
			if config.Value != "initial" {
				t.Fatalf("startup configuration changed while queued: %+v", config)
			}
		}
	}
	if !found {
		t.Fatal("startup lost its initial configuration")
	}
	successes := 0
	for i, body := range bodies {
		if body.Type == "app-started" {
			if body.Time < before || body.Time > after {
				t.Fatalf("startup timestamp moved to delivery: %+v, init [%d,%d]", body, before, after)
			}
			if scenario != "retry" || i >= 2 {
				successes++
			}
		}
		if i > 0 && (body.Seq < bodies[i-1].Seq || (body.Seq == bodies[i-1].Seq && body.Type != "app-started")) {
			t.Fatalf("sequence went backwards: %+v", bodies)
		}
	}
	if successes != 1 {
		t.Fatalf("startup successfully delivered %d times: %+v", successes, bodies)
	}
	// This client has metrics/logs disabled, so closing is a standalone event.
	if bodies[len(bodies)-1].Type != "app-closing" {
		t.Fatalf("session close missing: %+v", bodies)
	}
}
