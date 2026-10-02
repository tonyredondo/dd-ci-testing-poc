// Package minitracer owns native CI events; it has no APM integrations.
package minitracer

import (
	"context"
	"encoding/binary"
	"fmt"
	"maps"
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra/ext"
	"github.com/tonyredondo/dd-ci-testing-poc/propagation"
)

// Span is one mutable CI event. Finish takes an immutable snapshot once.
type Span struct {
	mu       sync.Mutex
	client   *Client
	identity propagation.Context
	content  tslvSpan
	finished bool
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
		s.content.Meta[key] = v
		delete(s.content.Metrics, key)
	case bool:
		s.content.Meta[key] = strconv.FormatBool(v)
		delete(s.content.Metrics, key)
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
			s.content.Meta[key] = strconv.FormatInt(v, 10)
			delete(s.content.Metrics, key)
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
			s.content.Meta[key] = strconv.FormatUint(v, 10)
			delete(s.content.Metrics, key)
		} else {
			s.setMetric(key, float64(v))
		}
	case float32:
		s.setMetric(key, float64(v))
	case float64:
		s.setMetric(key, v)
	default:
		s.content.Meta[key] = fmt.Sprint(v)
		delete(s.content.Metrics, key)
	}
}
func (s *Span) setMetric(k string, v float64) { s.content.Metrics[k] = v; delete(s.content.Meta, k) }
func (s *Span) Meta(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.content.Meta[key]
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
	content.Meta = maps.Clone(content.Meta)
	content.Metrics = maps.Clone(content.Metrics)
	content.Duration = cfg.time.UnixNano() - content.Start
	if content.Duration < 0 {
		content.Duration = 0
	}
	event := &ciVisibilityEvent{Type: content.Type, Version: 1, Content: content}
	takeID := func(key string) uint64 {
		v := content.Meta[key]
		delete(content.Meta, key)
		id, _ := strconv.ParseUint(v, 10, 64)
		return id
	}
	event.Content.SessionID = takeID("test_session_id")
	event.Content.ModuleID = takeID("test_module_id")
	event.Content.SuiteID = takeID("test_suite_id")
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
	s := &Span{client: client, identity: identity, content: tslvSpan{Name: name, Resource: name, Start: time.Now().UnixNano(), SpanID: identity.SpanID, TraceID: binary.BigEndian.Uint64(identity.TraceID[8:]), ParentID: parent, Meta: map[string]string{}, Metrics: map[string]float64{}}}
	if client != nil {
		s.content.Service = client.service
		for k, v := range client.tags {
			s.content.Meta[k] = v
		}
		if client.env != "" {
			s.content.Meta["env"] = client.env
		}
		if client.serviceVersion != "" {
			s.content.Meta["version"] = client.serviceVersion
		}
	}
	s.content.Meta["_dd.origin"] = "ciapp-test"
	s.content.Meta["_dd.p.tid"] = fmt.Sprintf("%016x", binary.BigEndian.Uint64(identity.TraceID[:8]))
	s.content.Metrics["process_id"] = float64(os.Getpid())
	for _, option := range options {
		option(s)
	}
	return s, propagation.WithContext(ctx, identity)
}
