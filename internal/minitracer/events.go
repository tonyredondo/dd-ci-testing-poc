// Derived from dd-trace-go v2.11.0-rc.1, Apache-2.0.
//
//go:generate env GOWORK=off go run github.com/tinylib/msgp@v1.6.4 -file=events.go -unexported -marshal=false -o=events_msgp.go -tests=false
package minitracer

import "github.com/tinylib/msgp/msgp"

type ciTestCyclePayload struct {
	Version  int32                        `msg:"version"`
	Metadata map[string]map[string]string `msg:"metadata"`
	Events   msgp.Raw                     `msg:"events"`
}
type ciTestCyclePayloadList []*ciTestCyclePayload
type ciVisibilityEvents []*ciVisibilityEvent
type ciVisibilityEvent struct {
	Type    string   `msg:"type"`
	Version int32    `msg:"version"`
	Content tslvSpan `msg:"content"`
}
type tslvSpan struct {
	SessionID     uint64             `msg:"test_session_id,omitempty"`    // identifier of this session
	ModuleID      uint64             `msg:"test_module_id,omitempty"`     // identifier of this module
	SuiteID       uint64             `msg:"test_suite_id,omitempty"`      // identifier of this suite
	CorrelationID string             `msg:"itr_correlation_id,omitempty"` // Correlation Id for Intelligent Test Runner transactions
	Name          string             `msg:"name"`                         // operation name
	Service       string             `msg:"service"`                      // service name (i.e. "grpc.server", "http.request")
	Resource      string             `msg:"resource"`                     // resource name (i.e. "/user?id=123", "SELECT * FROM users")
	Type          string             `msg:"type"`                         // protocol associated with the span (i.e. "web", "db", "cache")
	Start         int64              `msg:"start"`                        // span start time expressed in nanoseconds since epoch
	Duration      int64              `msg:"duration"`                     // duration of the span expressed in nanoseconds
	SpanID        uint64             `msg:"span_id,omitempty"`            // identifier of this span
	TraceID       uint64             `msg:"trace_id,omitempty"`           // lower 64-bits of the root span identifier
	ParentID      uint64             `msg:"parent_id,omitempty"`          // identifier of the span's direct parent
	Error         int32              `msg:"error"`                        // error status of the span; 0 means no errors
	Meta          map[string]string  `msg:"meta,omitempty"`               // arbitrary map of metadata
	Metrics       map[string]float64 `msg:"metrics,omitempty"`            // arbitrary map of numeric metrics
}
