package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// retryConfig is the resolved retry/timeout policy for one crawl cycle. It is
// built from config.CrawlerConfig in the manager and passed to fetchWithRetry
// so the retry logic is unit-testable without a *config.Config.
type retryConfig struct {
	// timeout bounds a single source's whole fetch (across all retries). 0 = no
	// per-crawl deadline (the shared HTTP client's 30s timeout is the only bound).
	timeout time.Duration
	// maxRetries is how many times a transient error is retried. 0 = no retries.
	maxRetries int
	// initialBackoff is the base delay before the first retry; later retries
	// back off exponentially (×2) with up to ±20% jitter. 0 = no delay.
	initialBackoff time.Duration
	// sleeper abstracts the delay between retries (test hook).
	sleeper func(ctx context.Context, d time.Duration) error
}

// defaultRetryConfig returns a no-op retry policy (timeout 0, 0 retries) so the
// manager's behavior is unchanged until config opts in.
func defaultRetryConfig() retryConfig {
	return retryConfig{sleeper: sleepContext}
}

// sleepContext sleeps for d, returning ctx.Err() early if the context is
// cancelled. It is the production sleeper.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// classifyFetchError decides whether an error from a fetch attempt is
// transient (worth retrying) and, after retries are exhausted, which crawl
// status it maps to.
//
// Transient: timeouts (context.DeadlineExceeded, url.Error with a timeout),
// connection resets / refused / EOF, and HTTP 5xx surfaced as a sentinel
// httpStatusError. Also any net.Error that reports Temporary()=true.
//
// Not transient (never retried): 4xx client errors, config/parse errors, and
// rate limiting (rate limiting has its own status and its own backoff at the
// upstream protocol level, so we don't add client-side retries on top).
type errorClass int

const (
	classTransient  errorClass = iota // retry with backoff
	classPermanent                    // do not retry → status "error"
	classRateLimit                    // do not retry → status "rate_limited"
)

func classifyFetchError(err error) errorClass {
	if err == nil {
		return classPermanent
	}
	// Rate limiting is never retried here; it is surfaced as its own status.
	if errors.Is(err, ErrRateLimited) {
		return classRateLimit
	}
	// Explicit HTTP status errors (set by the HTTP-layer fetchers via the
	// retryDo wrapper). 5xx is transient; everything else (4xx, 3xx, etc.) is
	// permanent.
	var hse *httpStatusError
	if errors.As(err, &hse) {
		if hse.code >= 500 && hse.code <= 599 {
			return classTransient
		}
		return classPermanent
	}
	// A url.Error wrapping a timeout (the shape net/http returns when a per-call
	// deadline fires) is transient.
	var ue *url.Error
	if errors.As(err, &ue) && ue.Timeout() {
		return classTransient
	}
	// DeadlineExceeded from our own per-crawl context, or any cancellable net
	// op that ran out of time.
	if errors.Is(err, context.DeadlineExceeded) {
		return classTransient
	}
	// Net errors flagged temporary (connection reset, refused, etc.).
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return classTransient
	}
	// Common low-level I/O transient strings from the stdlib and across
	// platforms. These are a safety net for error types we don't unwrap neatly.
	// Matched case-insensitively because the stdlib mixes casing
	// (e.g. io.ErrUnexpectedEOF → "unexpected EOF" vs "EOF").
	msg := strings.ToLower(err.Error())
	for _, s := range transientErrorSubstrings {
		if strings.Contains(msg, s) {
			return classTransient
		}
	}
	return classPermanent
}

// transientErrorSubstrings are lowercased message fragments that indicate a
// transient network condition across stdlib/platform error strings.
var transientErrorSubstrings = []string{
	"connection reset",
	"connection refused",
	"eof",
	"broken pipe",
	"no such host",
	"i/o timeout",
	"deadline exceeded",
	"timeout",
	"server closed",
	"transport connection",
	"tls: handshake",
}

// fetchWithRetry runs fetch up to maxRetries+1 times, retrying only on
// transient errors with exponential backoff. The whole sequence is bounded by
// rc.timeout (a per-crawl deadline). It returns:
//   - the fetch result on success,
//   - the final error (after retries) on failure,
//   - the error class of that final error, so the caller can map it to the
//     right crawl status without re-classifying.
func fetchWithRetry[T any](ctx context.Context, rc retryConfig, fetch func(context.Context) (T, error)) (T, error, errorClass) {
	var zero T

	// Apply the per-crawl deadline. If the parent ctx is already bounded
	// tighter, the shorter deadline wins (context.WithTimeout keeps the
	// earlier of parent deadline and our timeout).
	cctx := ctx
	cancel := func() {}
	if rc.timeout > 0 {
		cctx, cancel = context.WithTimeout(ctx, rc.timeout)
	}
	defer cancel()

	var lastErr error
	class := classPermanent
	attempts := rc.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if err := cctx.Err(); err != nil {
			// Per-crawl deadline elapsed while waiting between retries.
			return zero, err, classTransient
		}
		result, err := fetch(cctx)
		if err == nil {
			return result, nil, classPermanent
		}
		lastErr = err
		class = classifyFetchError(err)
		// Only transient errors are retried. Rate-limit and permanent errors
		// return immediately with their class.
		if class != classTransient {
			return zero, lastErr, class
		}
		// Last attempt — no point sleeping.
		if attempt == attempts-1 {
			break
		}
		backoff := backoffFor(rc.initialBackoff, attempt)
		if sleepErr := rc.sleeper(cctx, backoff); sleepErr != nil {
			// Cancelled while sleeping: surface as transient (deadline).
			return zero, fmt.Errorf("%w (during retry backoff): %v", sleepErr, lastErr), classTransient
		}
	}
	// Exhausted retries on a transient error → caller maps to network_error.
	return zero, lastErr, class
}

// backoffFor returns the delay before retry attempt+1, given the initial
// backoff and the (0-based) attempt number that just failed. It is
// exponential (initial × 2^attempt) with ±20% jitter to avoid synchronized
// retry storms against the same upstream.
func backoffFor(initial time.Duration, attempt int) time.Duration {
	if initial <= 0 || attempt < 0 {
		return 0
	}
	d := initial << attempt // initial × 2^attempt
	if d <= 0 {
		// Shift overflow — cap at a sane maximum.
		d = 30 * time.Second
	}
	// ±20% jitter: multiply by a factor in [0.8, 1.2).
	factor := 0.8 + 0.4*rand.Float64()
	jittered := time.Duration(float64(d) * factor)
	return jittered
}

// httpStatusError wraps an HTTP status code so classifyFetchError can decide
// transient (5xx) vs permanent (4xx) without re-parsing a message string.
type httpStatusError struct {
	code int
	msg  string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("http status %d: %s", e.code, e.msg)
}

// retryDo is an HTTP-layer helper that fetchers may use to wrap httpClient.Do
// so non-2xx responses are classified via httpStatusError rather than as plain
// fmt.Errorf strings. It runs a single request (no retries of its own — the
// caller's fetchWithRetry handles retrying the whole fetch, which is usually
// the desired granularity for crawl sources).
//
// On a non-2xx response it reads and discards the body (bounded) and returns an
// httpStatusError, so the upstream retry policy decides whether to retry.
func retryDo(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain up to 4KB of the body for a useful error message, then close.
		body := drainBounded(resp.Body, 4096)
		resp.Body.Close()
		return nil, &httpStatusError{code: resp.StatusCode, msg: body}
	}
	return resp, nil
}

// drainBounded reads up to limit bytes from r as a string, ignoring read errors
// (the body is only used to enrich an error message). It avoids unbounded reads
// of an error-response body on the resource-constrained device.
func drainBounded(r io.Reader, limit int) string {
	if r == nil {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return ""
	}
	if len(b) > limit {
		b = b[:limit]
	}
	return string(b)
}
