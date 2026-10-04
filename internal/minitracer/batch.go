package minitracer

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/cidelivery"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/citransport"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/telemetry"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/msgp/msgp"
)

// A full queue applies backpressure. Failed delivery preserves the older batch
// and rejects the incoming event explicitly, keeping the configured bound.
func (c *Client) add(event *ciEvent) {
	deadline := enqueueDeadline{deadline: time.Now().Add(c.timeout)}
	defer deadline.stop()
	select {
	case c.sendMu <- struct{}{}:
	default:
		if err := c.acquire(deadline.context()); err != nil {
			c.reject(err)
			return
		}
	}
	defer c.release()
	c.mu.Lock()
	// Msgsize is a cheap upper bound supplied by the CI schema. Include the
	// envelope so even a single large event cannot exceed the intake limit.
	size := eventSize(event)
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
	if full && !c.deferUntilIdle {
		if err := c.flushLocked(deadline.context()); err != nil {
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
	if flush && !c.deferUntilIdle {
		_ = c.flushLocked(deadline.context())
	}
}

// enqueueDeadline allocates a timer only when an enqueue must wait or deliver a
// batch. The deadline starts at entry, so waiting for another sender never gives
// a subsequent flush a new timeout budget.
type enqueueDeadline struct {
	deadline time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

func (d *enqueueDeadline) context() context.Context {
	if d.ctx == nil {
		d.ctx, d.cancel = context.WithDeadline(context.Background(), d.deadline)
	}
	return d.ctx
}

func (d *enqueueDeadline) stop() {
	if d.cancel != nil {
		d.cancel()
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

// Flush waits for buffered events to reach intake. In deferred mode an active
// test leaves delivery to its next idle checkpoint; it must not wait for itself.
func (c *Client) Flush(ctx context.Context) error {
	if c.deferUntilIdle {
		if err := ctx.Err(); err != nil {
			return err
		}
		return cidelivery.RunIfIdle(func() error { return c.flush(ctx) })
	}
	return c.flush(ctx)
}

func (c *Client) flush(ctx context.Context) error {
	if err := c.acquire(ctx); err != nil {
		return err
	}
	defer c.release()
	for {
		if err := c.flushLocked(ctx); err != nil {
			return err
		}
		c.mu.Lock()
		empty := len(c.events) == 0
		c.mu.Unlock()
		if empty {
			return nil
		}
	}
}
func (c *Client) flushLocked(ctx context.Context) error {
	c.mu.Lock()
	all := c.events
	if len(all) == 0 {
		c.mu.Unlock()
		return nil
	}
	count, batchBytes := len(all), c.queuedBytes
	if c.deferUntilIdle {
		count, batchBytes = c.nextBatchLocked()
	}
	batch := all[:count]
	c.events = all[count:]
	c.queuedBytes -= batchBytes
	c.mu.Unlock()
	serializationStart := time.Now()
	c.payload.Reset()
	c.payload.Grow(batchBytes + c.envelopeBytes)
	metadata, wireEvents := prepareCommonMetadata(c.metadata, batch)
	err := msgp.Encode(&c.payload, &testCycleBatch{Version: 1, Metadata: metadata, Events: wireEvents})
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
		c.events = all
		c.queuedBytes += batchBytes
	} else {
		if err != nil {
			c.discardBatchLocked(batch)
		} else {
			clear(batch)
		}
		if len(c.events) == 0 {
			c.events = all[:0]
		}
	}
	if c.payload.Cap() > citransport.TestCycleFlushBytes {
		c.payload = bytes.Buffer{}
	}
	return err
}

// nextBatchLocked preserves event order and the same count/byte thresholds when
// a deferred parallel group has accumulated more than one intake payload.
func (c *Client) nextBatchLocked() (count, size int) {
	for _, event := range c.events {
		bytes := eventSize(event)
		if count > 0 && (count == c.maxEvents || size+bytes+c.envelopeBytes > citransport.TestCycleFlushBytes) {
			break
		}
		count++
		size += bytes
	}
	return count, size
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
	defer c.removeConnectionCloser()
	if c.removeIdleFlush != nil {
		c.removeIdleFlush()
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	// Closure is terminal, including panic/signal shutdown. The runtime drains
	// its idle coordinator first; explicit clients still own final delivery.
	err := c.flush(ctx)
	if err != nil {
		c.mu.Lock()
		// If acquiring sendMu was canceled, the queued batch still needs to
		// be abandoned. An in-flight batch is owned by flushLocked instead.
		for len(c.events) > 0 {
			count := len(c.events)
			if c.deferUntilIdle {
				count, _ = c.nextBatchLocked()
			}
			c.discardBatchLocked(c.events[:count])
			c.events = c.events[count:]
		}
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
