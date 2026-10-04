// Package minitracer owns native CI events; it has no APM integrations.
package minitracer

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/ddtrace/ext"
	"github.com/tonyredondo/dd-ci-testing-poc/propagation"
)

// Span is mutable until Finish seals its maps for shared, read-only delivery.
type Span struct {
	mu           sync.Mutex
	client       *Client
	identity     propagation.Context
	content      eventContent
	finished     bool
	hierarchy    [3]string
	hierarchySet uint8
	common       *CommonTags
}
type SpanContext struct{ identity propagation.Context }

func (c *SpanContext) SpanID() uint64                { return c.identity.SpanID }
func (c *SpanContext) TraceID() uint64               { return binary.BigEndian.Uint64(c.identity.TraceID[8:]) }
func (c *SpanContext) Portable() propagation.Context { return c.identity }
func (s *Span) Context() *SpanContext                { return &SpanContext{identity: s.identity} }

type StartSpanOption func(*Span)
type FinishOption func(*finishConfig)
type finishConfig struct{ time time.Time }

func ResourceName(v string) StartSpanOption { return func(s *Span) { s.content.Resource = v } }
func SpanType(v string) StartSpanOption     { return func(s *Span) { s.content.Type = v } }
func StartTime(v time.Time) StartSpanOption { return func(s *Span) { s.content.Start = v.UnixNano() } }
func Tag(k string, v any) StartSpanOption   { return func(s *Span) { s.SetTag(k, v) } }
func FinishTime(v time.Time) FinishOption   { return func(c *finishConfig) { c.time = v } }

func (s *Span) SetTag(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	switch key {
	case ext.ManualKeep:
		return // CI events are always recorded, without a sampler.
	case ext.Error:
		switch v := value.(type) {
		case bool:
			if v {
				s.content.Error = 1
			} else {
				s.content.Error = 0
			}
		case error:
			if v != nil {
				s.content.Error = 1
				s.content.Meta[ext.ErrorMsg] = v.Error()
				s.content.Meta[ext.ErrorType] = fmt.Sprintf("%T", v)
				s.content.Meta[ext.ErrorStack] = string(debug.Stack())
			}
		}
		return
	}
	switch v := value.(type) {
	case string:
		s.setTextTag(key, v)
	case bool:
		s.setTextTag(key, strconv.FormatBool(v))
	case int:
		s.setMetric(key, float64(v))
	case int8:
		s.setMetric(key, float64(v))
	case int16:
		s.setMetric(key, float64(v))
	case int32:
		s.setMetric(key, float64(v))
	case int64:
		if v > (1<<53)-1 || v < -(1<<53)+1 {
			s.setTextTag(key, strconv.FormatInt(v, 10))
		} else {
			s.setMetric(key, float64(v))
		}
	case uint:
		s.setMetric(key, float64(v))
	case uint8:
		s.setMetric(key, float64(v))
	case uint16:
		s.setMetric(key, float64(v))
	case uint32:
		s.setMetric(key, float64(v))
	case uint64:
		if v > (1<<53)-1 {
			s.setTextTag(key, strconv.FormatUint(v, 10))
		} else {
			s.setMetric(key, float64(v))
		}
	case float32:
		s.setMetric(key, float64(v))
	case float64:
		s.setMetric(key, v)
	default:
		s.setTextTag(key, fmt.Sprint(v))
	}
}

// Text tags replace numeric metrics of the same name. Hierarchy IDs remain
// outside the metadata map so serialization never mutates finished spans.
func (s *Span) setTextTag(key, value string) {
	s.setMeta(key, value)
	delete(s.content.Metrics, key)
}

func hierarchyIndex(key string) int {
	switch key {
	case "test_session_id":
		return 0
	case "test_module_id":
		return 1
	case "test_suite_id":
		return 2
	}
	return -1
}
func (s *Span) setMeta(key, value string) {
	if i := hierarchyIndex(key); i >= 0 {
		s.hierarchy[i] = value
		s.hierarchySet |= 1 << i
		return
	}
	s.content.Meta[key] = value
}
func (s *Span) setMetric(key string, value float64) {
	if s.content.Metrics == nil {
		s.content.Metrics = make(map[string]float64)
	}
	s.content.Metrics[key] = value
	delete(s.content.Meta, key)
	if i := hierarchyIndex(key); i >= 0 {
		s.hierarchySet &^= 1 << i
		s.hierarchy[i] = ""
	}
}
func (s *Span) Meta(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := hierarchyIndex(key); i >= 0 {
		return s.hierarchy[i], s.hierarchySet&(1<<i) != 0
	}
	v, ok := s.content.Meta[key]
	if !ok && s.common != nil {
		if _, numeric := s.content.Metrics[key]; !numeric {
			v, ok = s.common.values[key]
		}
	}
	return v, ok
}
func (s *Span) Metric(key string) (float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.content.Metrics[key]
	return v, ok
}

// Finish seals the event and enqueues it exactly once. Hierarchy IDs are native
// CI fields, separate from distributed trace parentage.
func (s *Span) Finish(options ...FinishOption) {
	cfg := finishConfig{time: time.Now()}
	for _, option := range options {
		option(&cfg)
	}
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	content := s.content
	content.Duration = cfg.time.UnixNano() - content.Start
	if content.Duration < 0 {
		content.Duration = 0
	}
	event := &ciEvent{Type: content.Type, Version: 1, Content: content, common: s.common}
	event.Content.SessionID, _ = strconv.ParseUint(s.hierarchy[0], 10, 64)
	event.Content.ModuleID, _ = strconv.ParseUint(s.hierarchy[1], 10, 64)
	event.Content.SuiteID, _ = strconv.ParseUint(s.hierarchy[2], 10, 64)
	switch content.Type {
	case "test":
		event.Content.CorrelationID = content.Meta["itr_correlation_id"]
		event.Version = 2
		event.Content.ParentID = 0
	case "test_session_end", "test_module_end", "test_suite_end":
		event.Content.TraceID = 0
		event.Content.SpanID = 0
		event.Content.ParentID = 0
	default:
		event.Type = "span"
	}
	s.mu.Unlock()
	if s.client != nil {
		s.client.add(event)
	}
}
func newSpan(client *Client, ctx context.Context, name string, options ...StartSpanOption) (*Span, context.Context) {
	identity, ok := propagation.FromContext(ctx)
	parent := uint64(0)
	var err error
	if ok {
		parent = identity.SpanID
		identity, err = identity.Child()
	} else {
		identity, err = propagation.New()
	}
	if err != nil {
		panic("cannot generate CI trace identifiers")
	}
	capacity := len(options) + 3
	if client != nil {
		capacity += len(client.tags)
	}
	s := &Span{client: client, identity: identity, content: eventContent{Name: name, Resource: name, Start: time.Now().UnixNano(), SpanID: identity.SpanID, TraceID: binary.BigEndian.Uint64(identity.TraceID[8:]), ParentID: parent, Meta: make(map[string]string, capacity)}}
	if client != nil {
		s.content.Service = client.service
		for k, v := range client.tags {
			s.setMeta(k, v)
		}
		if client.env != "" {
			s.content.Meta["env"] = client.env
		}
		if client.serviceVersion != "" {
			s.content.Meta["version"] = client.serviceVersion
		}
	}
	s.content.Meta["_dd.origin"] = "ciapp-test"
	s.content.Meta["_dd.p.tid"] = hex.EncodeToString(identity.TraceID[:8])
	for _, option := range options {
		option(s)
	}
	return s, propagation.WithContext(ctx, identity)
}
