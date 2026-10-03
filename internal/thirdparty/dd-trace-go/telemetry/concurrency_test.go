package telemetry

import (
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry/internal"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry/internal/transport"
)

func runConcurrent(workers int, f func(int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range workers {
		wg.Go(func() { <-start; f(i) })
	}
	close(start)
	wg.Wait()
}

func TestMetricRegistrationIsCanonical(t *testing.T) {
	for _, kind := range []transport.MetricType{transport.CountMetric, transport.GaugeMetric, transport.RateMetric} {
		t.Run(string(kind), func(t *testing.T) {
			m := metrics{skipAllowlist: true}
			handles := make([]MetricHandle, 64)
			runConcurrent(len(handles), func(i int) {
				handles[i] = m.LoadOrStore(NamespaceCIVisibility, kind, "fixture", []string{"b:2", "a:1"})
			})
			for _, h := range handles {
				if h != handles[0] {
					t.Fatal("concurrent registration returned different handles")
				}
			}
			if kind == transport.RateMetric && handles[0].(*rate).intervalStart.Load() == nil {
				t.Fatal("rate interval was not initialized")
			}
		})
	}
}

// Each sample must land in exactly one collection, including submissions that
// overlap collection and the first registration of a metric.
func TestConcurrentMetricCollectionPreservesValues(t *testing.T) {
	const workers, iterations = 8, 64
	m := metrics{skipAllowlist: true}
	d := distributions{skipAllowlist: true, queueSize: internal.Range[int]{Min: 1024, Max: 1024}, pool: internal.NewSyncPool(func() []float64 { return make([]float64, 1024) })}
	handles := make([]MetricHandle, workers)
	seen := make([]int, workers*iterations)
	var countTotal float64
	collect := func() {
		if p := m.Payload(); p != nil {
			for _, s := range p.(transport.GenerateMetrics).Series {
				countTotal += s.Points[0][1].(float64)
			}
		}
		if p := d.Payload(); p != nil {
			for _, s := range p.(transport.Distributions).Series {
				for _, point := range s.Points {
					i := int(point)
					if i < 0 || i >= len(seen) {
						t.Errorf("unexpected distribution point %v", point)
						continue
					}
					seen[i]++
				}
			}
		}
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				collect()
				runtime.Gosched()
			}
		}
	}()
	runConcurrent(workers, func(worker int) {
		c := m.LoadOrStore(NamespaceCIVisibility, transport.CountMetric, "fixture", nil)
		handles[worker] = d.LoadOrStore(NamespaceCIVisibility, "fixture", nil)
		for i := range iterations {
			c.Submit(1)
			handles[worker].Submit(float64(worker*iterations + i))
		}
	})
	close(stop)
	<-done
	collect()
	if countTotal != workers*iterations {
		t.Fatalf("count total = %v, want %v", countTotal, workers*iterations)
	}
	for _, h := range handles {
		if h != handles[0] {
			t.Fatal("distribution registration returned different handles")
		}
	}
	for i, count := range seen {
		if count != 1 {
			t.Errorf("sample %d collected %d times, want once", i, count)
		}
	}
}

func TestGlobalRegistrationReplaysAndSwapsOnce(t *testing.T) {
	restore := MockClient(nil)
	defer restore()
	makeClient := func() Client {
		c, err := NewClient("fixture", "test", "1", ClientConfig{AgentURL: "http://fixture.invalid", HTTPClient: &http.Client{Transport: benchmarkHTTP{}}})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	first, second := makeClient(), makeClient()
	defer first.Close()
	defer second.Close()
	handles := make([]MetricHandle, 128)
	runConcurrent(len(handles), func(i int) {
		handles[i] = Count(NamespaceCIVisibility, "events_enqueued_for_serialization", nil)
		handles[i].Submit(2)
	})
	for _, h := range handles {
		if h != handles[0] {
			t.Fatal("startup created duplicate swappable handles")
		}
	}
	SwapClient(first)
	if got := first.Count(NamespaceCIVisibility, "events_enqueued_for_serialization", nil).Get(); got != 256 {
		t.Fatalf("startup replay = %v, want 256", got)
	}
	SwapClient(second)
	if h := Count(NamespaceCIVisibility, "events_enqueued_for_serialization", nil); h != handles[0] {
		t.Fatal("swapping client replaced the cached handle")
	}
	handles[0].Submit(7)
	if got := second.Count(NamespaceCIVisibility, "events_enqueued_for_serialization", nil).Get(); got != 7 {
		t.Fatalf("swapped handle submitted %v, want 7", got)
	}
}

func TestConcurrentLogCollectionPreservesCounts(t *testing.T) {
	const workers, iterations = 8, 1000
	logger := newLoggerBackend(8192)
	var total uint64
	collect := func() {
		if p := logger.Payload(); p != nil {
			for _, entry := range p.(transport.Logs).Logs {
				total += uint64(entry.Count)
			}
		}
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				collect()
				runtime.Gosched()
			}
		}
	}()
	runConcurrent(workers, func(int) {
		for range iterations {
			logger.Add(NewRecord(LogWarn, "fixture"), WithTags([]string{"ci:true"}))
		}
	})
	close(stop)
	<-done
	collect()
	if total != workers*iterations {
		t.Fatalf("log count = %d, want %d", total, workers*iterations)
	}
	if got := logger.distinctLogs.Load(); got != 0 {
		t.Fatalf("distinct logs after collection = %d", got)
	}
	if logger.Payload() != nil {
		t.Fatal("collection emitted a duplicate batch")
	}
}

func TestLogLimitAndStacktracePreserved(t *testing.T) {
	logger := newLoggerBackend(2)
	logger.Add(NewRecord(LogWarn, "first"), WithTags([]string{"ci:true"}), WithStacktrace())
	logger.Add(NewRecord(LogWarn, "first"), WithTags([]string{"ci:true"}), WithStacktrace())
	logger.Add(NewRecord(LogError, "second"))
	for range 4 {
		logger.Add(NewRecord(LogError, "dropped"))
	}
	logs := logger.Payload().(transport.Logs).Logs
	if len(logs) != 3 {
		t.Fatalf("log entries = %d, want first, second and one limit warning", len(logs))
	}
	var warnings int
	for _, entry := range logs {
		switch {
		case entry.Message == "first":
			if entry.Count != 2 || entry.Tags != "ci:true" || !strings.Contains(entry.StackTrace, "TestLogLimitAndStacktracePreserved") {
				t.Errorf("first log lost count/tags/caller stack: %+v", entry)
			}
		case strings.Contains(entry.Message, "exceeded maximum"):
			warnings++
			if entry.Count != 1 || entry.StackTrace == "" {
				t.Errorf("invalid limit warning: %+v", entry)
			}
		case entry.Message != "second":
			t.Errorf("unexpected log: %+v", entry)
		}
	}
	if warnings != 1 {
		t.Fatalf("limit warnings = %d, want 1", warnings)
	}
	logger.Add(NewRecord(LogWarn, "next"))
	if got := logger.Payload().(transport.Logs).Logs; len(got) != 1 || got[0].Message != "next" {
		t.Fatalf("logger did not resume after collection: %+v", got)
	}
}
