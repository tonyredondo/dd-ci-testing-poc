package minitracer

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/citransport"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/telemetry"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/msgp/msgp"
)

// A full queue applies backpressure. Failed delivery preserves the older batch
// and rejects the incoming event explicitly, keeping the configured bound.
func (c *Client) add(event *ciEvent) {
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
	c.mu.Unlock()
}

// DroppedEvents reports rejected events and events abandoned at final closure.
// This event count is separate from the payload-drop telemetry counter.
func (c *Client) DroppedEvents() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

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
	err := msgp.Encode(&c.payload, &testCycleBatch{Version: 1, Metadata: c.metadata, Events: batch})
	if err == nil {
		telemetry.EndpointPayloadEventsCount(telemetry.TestCycleEndpointType, float64(len(batch)))
		telemetry.EndpointPayloadBytes(telemetry.TestCycleEndpointType, float64(c.payload.Len()))
		telemetry.EndpointEventsSerializationMs(telemetry.TestCycleEndpointType, float64(time.Since(serializationStart).Milliseconds()))
		err = c.transport.Send(ctx, c.payload.Bytes())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErr = err
	if err != nil && !c.closed {
		c.events = batch
		c.queuedBytes = batchBytes
	} else {
		if err != nil {
			c.discardBatchLocked(batch)
		} else {
			clear(batch)
		}
		c.events = batch[:0]
	}
	if c.payload.Cap() > citransport.TestCycleFlushBytes {
		c.payload = bytes.Buffer{}
	}
	return err
}

// discardBatchLocked records one abandoned payload, regardless of its event
// count. The caller holds mu; retryable flush failures never call this method.
func (c *Client) discardBatchLocked(batch []*ciEvent) {
	if len(batch) == 0 {
		return
	}
	c.dropped += uint64(len(batch))
	telemetry.EndpointPayloadDropped(telemetry.TestCycleEndpointType)
	clear(batch)
}

// Close seals the client, performs a final flush and releases connections.
// An unsuccessful final flush abandons its batch once. Earlier Flush failures
// remain retryable; a closed client cannot later resend an abandoned payload.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	err := c.Flush(ctx)
	if err != nil {
		c.mu.Lock()
		// If acquiring sendMu was canceled, the queued batch still needs to
		// be abandoned. An in-flight batch is owned by flushLocked instead.
		c.discardBatchLocked(c.events)
		c.events = nil
		c.queuedBytes = 0
		c.lastErr = err
		c.mu.Unlock()
	}
	c.transport.CloseIdleConnections()
	return err
}

// LastError returns the most recent delivery or rejection error.
func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}
