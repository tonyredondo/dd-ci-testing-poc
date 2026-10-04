// Package minitracer records native CI events independently of the APM tracer.
package minitracer

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/cidelivery"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/citransport"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/globalconfig"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/msgp/msgp"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/version"
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
	// DeferUntilIdle buffers events while instrumented tests run. Payload limits
	// still apply; the queue can grow until the current parallel group finishes.
	DeferUntilIdle bool
}

// Client buffers native events. Event ownership transfers on Finish. Flushes
// serialize; a failed batch stays queued and can be retried by a later Flush.
type Client struct {
	mu                     sync.Mutex
	sendMu                 chan struct{}
	events                 []*ciEvent
	transport              *citransport.Transport
	service, env           string
	serviceVersion         string
	queuedBytes            int
	envelopeBytes          int
	tags                   map[string]string
	metadata               map[string]map[string]string
	maxEvents              int
	timeout                time.Duration
	closed                 bool
	lastErr                error
	dropped                uint64
	payload                bytes.Buffer // Protected by sendMu; released after oversized batches.
	deferUntilIdle         bool
	removeIdleFlush        func()
	removeConnectionCloser func()
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
	if c.DeferUntilIdle {
		c.Transport.CloseIdleAfterSend = true
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
	client := &Client{
		sendMu:         make(chan struct{}, 1),
		transport:      transport,
		service:        c.Service,
		env:            c.Env,
		serviceVersion: c.ServiceVersion,
		envelopeBytes:  (&testCyclePayload{Version: 1, Metadata: metadata}).Msgsize() + msgp.ArrayHeaderSize,
		tags:           tags,
		metadata:       metadata,
		maxEvents:      c.MaxEvents,
		timeout:        c.FlushTimeout,
		deferUntilIdle: c.DeferUntilIdle,
	}
	client.removeConnectionCloser = cidelivery.RegisterConnectionCloser(transport.CloseOwnedIdleConnections)
	if c.DeferUntilIdle {
		client.removeIdleFlush = cidelivery.Register(func() {
			client.mu.Lock()
			empty := len(client.events) == 0
			client.mu.Unlock()
			if empty {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), client.timeout)
			defer cancel()
			_ = client.flush(ctx)
		})
	}
	return client, nil
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
