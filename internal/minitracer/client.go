package minitracer

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	infra "github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra/env"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra/globalconfig"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra/log"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/ciinfra/version"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/citransport"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/civisibility/constants"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/civisibility/utils"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/civisibility/utils/telemetry"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/msgp"
)

const Version = version.Number

// Config is explicit so library consumers can avoid reading process state.
type Config struct {
	Service, Env string
	// ServiceVersion is the tested service version (DD_VERSION), not this library version.
	ServiceVersion string
	Tags           map[string]string
	Metadata       map[string]map[string]string
	Transport      citransport.Config
	MaxEvents      int
	FlushTimeout   time.Duration
}

// Client buffers native events. Event ownership transfers on Finish. Flushes
// serialize; a failed batch stays queued and can be retried by a later Flush.
type Client struct {
	mu             sync.Mutex
	sendMu         chan struct{}
	events         []*ciVisibilityEvent
	transport      *citransport.Transport
	service, env   string
	serviceVersion string
	queuedBytes    int
	envelopeBytes  int
	tags           map[string]string
	metadata       map[string]map[string]string
	maxEvents      int
	timeout        time.Duration
	closed         bool
	lastErr        error
	dropped        uint64
	payload        bytes.Buffer // Protected by sendMu; released after oversized batches.
}

func New(c Config) (*Client, error) {
	if c.MaxEvents == 0 {
		c.MaxEvents = 1000
	}
	if c.MaxEvents < 1 || c.MaxEvents > 100000 {
		return nil, errors.New("invalid event batch capacity")
	}
	if c.FlushTimeout == 0 {
		c.FlushTimeout = 10 * time.Second
	}
	if c.FlushTimeout < 0 {
		return nil, errors.New("invalid flush timeout")
	}
	if c.Transport.Version == "" {
		c.Transport.Version = Version
	}
	transport, err := citransport.New(c.Transport)
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for k, v := range c.Tags {
		tags[k] = v
	}
	metadata := map[string]map[string]string{"*": {"language": "go", "runtime-id": globalconfig.RuntimeID(), "library_version": Version}}
	if c.Env != "" {
		metadata["*"]["env"] = c.Env
	}
	for kind, values := range c.Metadata {
		dst := metadata[kind]
		if dst == nil {
			dst = map[string]string{}
			metadata[kind] = dst
		}
		for k, v := range values {
			dst[k] = v
		}
	}
	return &Client{
		sendMu: make(chan struct{}, 1), transport: transport,
		service: c.Service, env: c.Env, serviceVersion: c.ServiceVersion,
		envelopeBytes: (&ciTestCyclePayload{Version: 1, Metadata: metadata}).Msgsize() + msgp.ArrayHeaderSize,
		tags:          tags, metadata: metadata, maxEvents: c.MaxEvents, timeout: c.FlushTimeout,
	}, nil
}
func (c *Client) StartSpan(ctx context.Context, name string, options ...StartSpanOption) (*Span, context.Context) {
	return newSpan(c, ctx, name, options...)
}
func (c *Client) acquire(ctx context.Context) error {
	select {
	case c.sendMu <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *Client) release() { <-c.sendMu }

// A full queue applies backpressure. Failed delivery preserves the older batch
// and rejects the incoming event explicitly, keeping the configured bound.
func (c *Client) add(event *ciVisibilityEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	if err := c.acquire(ctx); err != nil {
		c.reject(err)
		return
	}
	defer c.release()
	c.mu.Lock()
	// Msgsize is a cheap upper bound supplied by the CI schema. Include the
	// envelope so even a single large event cannot exceed the intake limit.
	size := event.Msgsize()
	closed := c.closed
	oversized := size+c.envelopeBytes > citransport.TestCycleMaxPayloadBytes
	full := len(c.events) >= c.maxEvents || (len(c.events) > 0 && c.queuedBytes+size+c.envelopeBytes > citransport.TestCycleFlushBytes)
	c.mu.Unlock()
	if closed {
		c.reject(errors.New("event finished after CI client closed"))
		return
	}
	if oversized {
		c.reject(errors.New("CI event exceeds the test-cycle payload limit"))
		return
	}
	if full {
		if err := c.flushLocked(ctx); err != nil {
			c.reject(err)
			return
		}
	}
	c.mu.Lock()
	if c.closed {
		c.lastErr = errors.New("event finished after CI client closed")
		c.dropped++
		telemetry.EndpointPayloadDropped(telemetry.TestCycleEndpointType)
		c.mu.Unlock()
		return
	}
	telemetry.EventsEnqueueForSerialization()
	c.events = append(c.events, event)
	c.queuedBytes += size
	flush := c.queuedBytes+c.envelopeBytes >= citransport.TestCycleFlushBytes
	c.mu.Unlock()
	if flush {
		_ = c.flushLocked(ctx)
	}
}
func (c *Client) reject(err error) {
	c.mu.Lock()
	c.lastErr = err
	c.dropped++
	telemetry.EndpointPayloadDropped(telemetry.TestCycleEndpointType)
	c.mu.Unlock()
}

// DroppedEvents reports events rejected after closure or while delivery failed
// with a full queue. Successful later flushes do not erase this counter.
func (c *Client) DroppedEvents() uint64 { c.mu.Lock(); defer c.mu.Unlock(); return c.dropped }

// Flush waits for this client's buffered events to reach the configured intake.
func (c *Client) Flush(ctx context.Context) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	return c.flushLocked(ctx)
}
func (c *Client) flushLocked(ctx context.Context) error {
	c.mu.Lock()
	batch := c.events
	if len(batch) == 0 {
		c.mu.Unlock()
		return nil
	}
	batchBytes := c.queuedBytes
	c.events = nil
	c.queuedBytes = 0
	c.mu.Unlock()
	serializationStart := time.Now()
	c.payload.Reset()
	c.payload.Grow(batchBytes + c.envelopeBytes)
	err := msgp.Encode(&c.payload, &ciTestCycleBatch{Version: 1, Metadata: c.metadata, Events: batch})
	if err == nil {
		telemetry.EndpointPayloadEventsCount(telemetry.TestCycleEndpointType, float64(len(batch)))
		telemetry.EndpointPayloadBytes(telemetry.TestCycleEndpointType, float64(c.payload.Len()))
		telemetry.EndpointEventsSerializationMs(telemetry.TestCycleEndpointType, float64(time.Since(serializationStart).Milliseconds()))
		err = c.transport.Send(ctx, c.payload.Bytes())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = err
	if err != nil {
		c.events = batch
		c.queuedBytes = batchBytes
	} else {
		clear(batch)
		c.events = batch[:0]
	}
	if c.payload.Cap() > citransport.TestCycleFlushBytes {
		c.payload = bytes.Buffer{}
	}
	return err
}

// Close seals the client, performs a final flush and releases connections.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	err := c.Flush(ctx)
	c.transport.CloseIdleConnections()
	return err
}
func (c *Client) LastError() error { c.mu.Lock(); defer c.mu.Unlock(); return c.lastErr }

var active atomic.Pointer[Client]

type StartOption func(*Config)

func WithService(v string) StartOption { return func(c *Config) { c.Service = v } }

// Start selects the runtime used by the extracted testing hooks.
func Start(options ...StartOption) {
	agentless := infra.BoolEnv("DD_CIVISIBILITY_AGENTLESS_ENABLED", false)
	endpoint := ""
	var clientConfig citransport.Config
	if agentless {
		base := env.Get("DD_CIVISIBILITY_AGENTLESS_URL")
		if base == "" {
			site := env.Get("DD_SITE")
			if site == "" {
				site = "datadoghq.com"
			}
			base = "https://citestcycle-intake." + site
		}
		endpoint = strings.TrimRight(base, "/") + "/api/v2/citestcycle"
		clientConfig.Agentless = true
		clientConfig.APIKey = env.Get("DD_API_KEY")
	} else {
		agent := infra.AgentURLFromEnv()
		if agent.Scheme == "unix" {
			clientConfig.HTTPClient = infra.UDSClient(agent.Path, 10*time.Second)
			agent = infra.UnixDataSocketURL(agent.Path)
		}
		endpoint = strings.TrimRight(agent.String(), "/") + "/evp_proxy/v2/api/v2/citestcycle"
	}
	clientConfig.Endpoint = endpoint
	config := Config{Service: env.Get("DD_SERVICE"), Env: env.Get("DD_ENV"), ServiceVersion: env.Get("DD_VERSION"), Transport: clientConfig, Tags: infra.ParseTagString(env.Get("DD_TAGS"))}
	for _, option := range options {
		option(&config)
	}
	if session, ok := utils.GetCITags()[constants.TestSessionName]; ok {
		config.Metadata = map[string]map[string]string{}
		for _, kind := range []string{"test", "test_session_end", "test_module_end", "test_suite_end"} {
			config.Metadata[kind] = map[string]string{"test_session.name": session}
		}
	}
	if config.Service == "" {
		config.Service = "go.test"
	}
	client, err := New(config)
	if err != nil {
		log.Error("CI mini tracer could not start: %s", err.Error())
		return
	}
	active.Store(client)
}
func StartSpanFromContext(ctx context.Context, name string, options ...StartSpanOption) (*Span, context.Context) {
	return newSpan(active.Load(), ctx, name, options...)
}
func Flush() {
	if c := active.Load(); c != nil {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		if err := c.Flush(ctx); err != nil {
			log.Error("CI event flush failed: %s", err.Error())
		}
	}
}
func Stop() {
	if c := active.Swap(nil); c != nil {
		ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			log.Error("CI event close failed: %s", err.Error())
		}
		if dropped := c.DroppedEvents(); dropped != 0 {
			log.Error("CI mini tracer rejected %d events", dropped)
		}
	}
}

// EndpointForAgent constructs the native EVP endpoint from an agent base URL.
func EndpointForAgent(agent *url.URL) string {
	return strings.TrimRight(agent.String(), "/") + "/evp_proxy/v2/api/v2/citestcycle"
}
