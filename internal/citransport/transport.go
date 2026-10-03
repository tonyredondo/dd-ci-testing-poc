// Package citransport sends native CI Visibility payloads without an APM SDK.
package citransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/bazel"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/telemetry"
)

// TestCycleMaxPayloadBytes and TestCycleFlushBytes match the CI Visibility
// writer in dd-trace-go. The limit applies to uncompressed MessagePack, including
// the envelope; gzip does not make an oversized intake payload acceptable.
const (
	TestCycleMaxPayloadBytes = 5 * 1024 * 1024
	TestCycleFlushBytes      = TestCycleMaxPayloadBytes / 2
)

// Config contains only the destination and delivery policy. Endpoint is the
// complete test-cycle URL. Credentials never appear in returned errors.
type Config struct {
	Endpoint   string
	Agentless  bool
	APIKey     string
	Version    string
	HTTPClient *http.Client
	Attempts   int
	RetryDelay time.Duration
}
type Transport struct {
	config Config
	client *http.Client
}

// New validates configuration before any event can be accepted.
func New(c Config) (*Transport, error) {
	u, err := url.Parse(c.Endpoint)
	if !bazel.IsPayloadFilesModeEnabled() && (err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil) {
		return nil, errors.New("invalid CI Visibility endpoint")
	}
	if !bazel.IsPayloadFilesModeEnabled() && c.Agentless && c.APIKey == "" {
		return nil, errors.New("agentless CI Visibility requires an API key")
	}
	if c.Attempts == 0 {
		c.Attempts = 3
	}
	if c.Attempts < 1 || c.Attempts > 10 {
		return nil, errors.New("invalid delivery attempts")
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = 100 * time.Millisecond
	}
	if c.RetryDelay < 0 {
		return nil, errors.New("invalid retry delay")
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// A copy preserves the caller's client. Never forward API credentials through
	// redirects, including custom headers not protected by net/http's defaults.
	copyClient := *client
	if copyClient.Timeout <= 0 {
		copyClient.Timeout = 10 * time.Second
	}
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Transport{config: c, client: &copyClient}, nil
}

// Send retries transient failures using the same immutable payload. Context
// cancellation bounds both requests and backoff; permanent 4xx responses fail.
func (t *Transport) Send(ctx context.Context, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	compression := telemetry.UncompressedRequestCompressedType
	if t.config.Agentless {
		compression = telemetry.CompressedRequestCompressedType
	}
	telemetry.EndpointPayloadRequests(telemetry.TestCycleEndpointType, compression)
	if len(payload) > TestCycleMaxPayloadBytes {
		return errors.New("CI test-cycle payload exceeds the intake limit")
	}
	if bazel.IsPayloadFilesModeEnabled() {
		data, err := bazel.MsgpackToJSON(payload)
		if err != nil {
			return err
		}
		return bazel.WritePayloadFile(bazel.PayloadKindTests, data)
	}
	body := payload
	if t.config.Agentless {
		compressor := gzipCompressors.Get().(*gzipCompressor)
		defer releaseCompressor(compressor)
		compressor.buffer.Reset()
		compressor.writer.Reset(&compressor.buffer)
		if _, err := compressor.writer.Write(payload); err != nil {
			return err
		}
		if err := compressor.writer.Close(); err != nil {
			return err
		}
		body = compressor.buffer.Bytes()
	}
	// net/http may close request bodies asynchronously, even after Do returns.
	// Seal every reader before returning the payload or compressor to its owner.
	owner := &bodyOwner{}
	defer owner.seal()
	var last error
	for attempt := 0; attempt < t.config.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := t.newRequest(ctx, body, owner)
		if err != nil {
			return err
		}
		requestStart := time.Now()
		resp, err := t.client.Do(req)
		telemetry.EndpointPayloadRequestsMs(telemetry.TestCycleEndpointType, float64(time.Since(requestStart).Milliseconds()))
		retry := false
		if err != nil {
			telemetry.EndpointPayloadRequestsErrors(telemetry.TestCycleEndpointType, telemetry.NetworkErrorType)
			last = errors.New("CI Visibility request failed")
			retry = true
		} else {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			telemetry.EndpointPayloadRequestsErrors(telemetry.TestCycleEndpointType, telemetry.GetErrorTypeFromStatusCode(resp.StatusCode))
			last = fmt.Errorf("CI Visibility endpoint returned HTTP %d", resp.StatusCode)
			retry = resp.StatusCode == 429 || resp.StatusCode >= 500
			if resp.StatusCode == 429 && attempt+1 < t.config.Attempts {
				if seconds, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil && seconds >= 0 && seconds <= 60 {
					if err = wait(ctx, time.Duration(seconds)*time.Second); err != nil {
						return err
					}
					continue
				}
			}
		}
		if !retry || attempt+1 == t.config.Attempts {
			return last
		}
		if err = wait(ctx, t.config.RetryDelay*time.Duration(1<<attempt)); err != nil {
			return err
		}
	}
	return last
}
func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// CloseIdleConnections releases this transport's idle connections after flush.
func (t *Transport) CloseIdleConnections() { t.client.CloseIdleConnections() }
