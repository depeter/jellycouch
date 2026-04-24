// Package httpx provides small helpers for HTTP calls — retry with backoff
// for transient failures and classification of error-vs-permanent responses.
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"time"
)

// RetryPolicy configures the retry loop. Zero-value is a reasonable default:
// 3 attempts, 200ms base backoff up to 2s, full jitter.
type RetryPolicy struct {
	MaxAttempts int           // total tries including the first; 0 or 1 disables retry
	BaseDelay   time.Duration // backoff start, doubled each retry
	MaxDelay    time.Duration // cap on the backoff interval
}

// defaults returns a RetryPolicy with zero-value fields filled in.
func (p RetryPolicy) defaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 200 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 2 * time.Second
	}
	return p
}

// Do executes req via client, retrying on transient errors (connection errors,
// 429, or 5xx). The caller is responsible for closing resp.Body on the final
// returned response. If all attempts fail the last error/response is returned.
func Do(ctx context.Context, client *http.Client, req *http.Request, policy RetryPolicy) (*http.Response, error) {
	policy = policy.defaults()
	var lastErr error
	var lastResp *http.Response
	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := backoff(policy, attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		// Clone the request so retries don't reuse a consumed body.
		r := req.Clone(ctx)
		resp, err := client.Do(r)
		if err == nil && !isTransientStatus(resp.StatusCode) {
			return resp, nil
		}
		// Transient — discard body and retry.
		if resp != nil {
			resp.Body.Close()
			lastResp = resp
		}
		lastErr = err
		if err != nil && !isTransientErr(err) {
			return nil, err
		}
		slog.Debug("http retry", "attempt", attempt+1, "max", policy.MaxAttempts, "err", err)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

func backoff(p RetryPolicy, attempt int) time.Duration {
	d := p.BaseDelay << (attempt - 1)
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	// Full jitter: random in [0, d].
	return time.Duration(rand.Int63n(int64(d) + 1))
}

func isTransientStatus(code int) bool {
	return code == http.StatusTooManyRequests || code == http.StatusServiceUnavailable ||
		code == http.StatusBadGateway || code == http.StatusGatewayTimeout
}

func isTransientErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	// Only net.Error (DNS, connection reset, timeout, etc.) is considered
	// transient. URL parse errors, schema errors, and other permanent
	// failures fall through to false so we don't waste retries on them.
	var netErr net.Error
	return errors.As(err, &netErr)
}
