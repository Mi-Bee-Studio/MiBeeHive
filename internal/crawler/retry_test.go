package crawler

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingSleeper is a test sleeper that records the requested delays without
// actually sleeping, so retry tests are fast. It signals each requested delay on
// sleptCh so a test can assert the backoff schedule.
type recordingSleeper struct {
	mu    sync.Mutex
	delim []time.Duration
}

func (r *recordingSleeper) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	r.delim = append(r.delim, d)
	r.mu.Unlock()
	return nil
}

func (r *recordingSleeper) delays() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]time.Duration, len(r.delim))
	copy(out, r.delim)
	return out
}

func TestClassifyFetchError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want errorClass
	}{
		{"nil", nil, classPermanent},
		{"rate limit", ErrRateLimited, classRateLimit},
		{"http 500", &httpStatusError{code: 500, msg: "boom"}, classTransient},
		{"http 503", &httpStatusError{code: 503, msg: "unavailable"}, classTransient},
		{"http 404", &httpStatusError{code: 404, msg: "nope"}, classPermanent},
		{"http 401", &httpStatusError{code: 401, msg: "nope"}, classPermanent},
		{"url timeout", &url.Error{Op: "Get", URL: "x", Err: timeoutErr{}}, classTransient},
		{"deadline exceeded", context.DeadlineExceeded, classTransient},
		{"net op timeout", &net.OpError{Op: "read", Err: timeoutErr{}}, classTransient},
		{"connection reset string", errors.New("read tcp: connection reset by peer"), classTransient},
		{"connection refused string", errors.New("dial tcp: connection refused"), classTransient},
		{"eof string", errors.New("unexpected EOF"), classTransient},
		{"no such host string", errors.New("dial tcp: lookup x: no such host"), classTransient},
		{"config error", errors.New("invalid source url"), classPermanent},
		{"parse error", errors.New("decoding response: invalid character"), classPermanent},
		{"rate limit message (fingerprint track)", errors.New(`source https://api.github.com: HTTP 403: {"message":"API rate limit exceeded"} (rate limit)`), classRateLimit},
		{"rate limit message no tag", errors.New("source x: HTTP 429: rate limit exceeded"), classRateLimit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyFetchError(c.err); got != c.want {
				t.Errorf("classifyFetchError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// timeoutErr satisfies the net package's timeout interface without pulling in a
// real deadline. It's used to exercise the url.Error/OpError branches.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestFetchWithRetry_TransientThenSuccess(t *testing.T) {
	// The fetch fails twice with a transient error, then succeeds. The retry
	// loop must call it 3 times total and return the successful result.
	var calls int32
	fetch := func(ctx context.Context) (string, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return "", &httpStatusError{code: 503, msg: "unavailable"}
		}
		return "ok", nil
	}
	slp := &recordingSleeper{}
	rc := retryConfig{maxRetries: 3, initialBackoff: 100 * time.Millisecond, sleeper: slp.sleep}

	result, err, class := fetchWithRetry(context.Background(), rc, fetch)
	if err != nil {
		t.Fatalf("expected success, got err: %v (class %v)", err, class)
	}
	if result != "ok" {
		t.Fatalf("result = %q, want ok", result)
	}
	if calls != 3 {
		t.Errorf("fetch called %d times, want 3 (2 transient retries + success)", calls)
	}
	if got := len(slp.delays()); got != 2 {
		t.Errorf("slept %d times, want 2", got)
	}
}

func TestFetchWithRetry_PermanentErrorNotRetried(t *testing.T) {
	// A 404 (permanent) must NOT be retried — one call, immediate return.
	var calls int32
	fetch := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", &httpStatusError{code: 404, msg: "not found"}
	}
	slp := &recordingSleeper{}
	rc := retryConfig{maxRetries: 3, initialBackoff: 100 * time.Millisecond, sleeper: slp.sleep}

	_, err, class := fetchWithRetry(context.Background(), rc, fetch)
	if err == nil {
		t.Fatal("expected error")
	}
	if class != classPermanent {
		t.Errorf("class = %v, want classPermanent", class)
	}
	if calls != 1 {
		t.Errorf("permanent error retried: calls=%d, want 1", calls)
	}
	if len(slp.delays()) != 0 {
		t.Errorf("permanent error should not sleep, slept %v", slp.delays())
	}
}

func TestFetchWithRetry_RateLimitNotRetried(t *testing.T) {
	// Rate limiting has its own status and must not be retried client-side.
	var calls int32
	fetch := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", ErrRateLimited
	}
	rc := retryConfig{maxRetries: 3, initialBackoff: 100 * time.Millisecond, sleeper: (&recordingSleeper{}).sleep}

	_, err, class := fetchWithRetry(context.Background(), rc, fetch)
	if err == nil {
		t.Fatal("expected error")
	}
	if class != classRateLimit {
		t.Errorf("class = %v, want classRateLimit", class)
	}
	if calls != 1 {
		t.Errorf("rate-limit error retried: calls=%d, want 1", calls)
	}
}

func TestFetchWithRetry_ExhaustsRetriesThenNetworkError(t *testing.T) {
	// A persistent transient failure (always 503) exhausts maxRetries and the
	// final class is classTransient — which the manager maps to network_error.
	var calls int32
	fetch := func(ctx context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", &httpStatusError{code: 503, msg: "unavailable"}
	}
	slp := &recordingSleeper{}
	rc := retryConfig{maxRetries: 2, initialBackoff: 100 * time.Millisecond, sleeper: slp.sleep}

	_, err, class := fetchWithRetry(context.Background(), rc, fetch)
	if err == nil {
		t.Fatal("expected error")
	}
	if class != classTransient {
		t.Errorf("class = %v, want classTransient (network_error after retries)", class)
	}
	// 2 retries → 3 total attempts.
	if calls != 3 {
		t.Errorf("calls=%d, want 3", calls)
	}
	if got := len(slp.delays()); got != 2 {
		t.Errorf("slept %d times, want 2", got)
	}
}

func TestFetchWithRetry_PerCrawlTimeout(t *testing.T) {
	// A per-crawl timeout must bound the whole fetch. We simulate a fetch that
	// never succeeds; the timeout (plus a fast sleeper that still honors ctx)
	// must cut it off well under the maxRetries schedule.
	fetch := func(ctx context.Context) (string, error) {
		// Block until the per-crawl ctx is cancelled, then report it.
		<-ctx.Done()
		return "", ctx.Err()
	}
	// Real sleeper so ctx cancellation is observed.
	rc := retryConfig{
		timeout:        50 * time.Millisecond,
		maxRetries:     10,
		initialBackoff: 1 * time.Second, // would be far too long if timeout didn't fire
		sleeper:        sleepContext,
	}
	start := time.Now()
	_, err, class := fetchWithRetry(context.Background(), rc, fetch)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if class != classTransient {
		t.Errorf("class = %v, want classTransient", class)
	}
	// Must return well under what 10 retries at 1s+ backoff would take.
	if elapsed > 2*time.Second {
		t.Errorf("per-crawl timeout did not bound the fetch: elapsed=%v", elapsed)
	}
}

func TestBackoffFor_Exponential(t *testing.T) {
	initial := 100 * time.Millisecond
	for attempt := 0; attempt < 5; attempt++ {
		d := backoffFor(initial, attempt)
		base := initial << attempt
		// ±20% jitter → [0.8×base, 1.2×base).
		if d < time.Duration(float64(base)*0.79) || d > time.Duration(float64(base)*1.21) {
			t.Errorf("attempt %d: backoff=%v base=%v (out of ±20%%)", attempt, d, base)
		}
	}
	if backoffFor(0, 3) != 0 {
		t.Error("backoffFor with zero initial should be 0")
	}
}

func TestRetryConfigFromCfg(t *testing.T) {
	t.Run("populated", func(t *testing.T) {
		cfg := makeTestConfig()
		cfg.Crawler.FetchTimeout = "90s"
		cfg.Crawler.MaxRetries = 5
		cfg.Crawler.RetryInitialBackoff = "500ms"
		rc := retryConfigFromCfg(cfg)
		if rc.timeout != 90*time.Second {
			t.Errorf("timeout=%v, want 90s", rc.timeout)
		}
		if rc.maxRetries != 5 {
			t.Errorf("maxRetries=%v, want 5", rc.maxRetries)
		}
		if rc.initialBackoff != 500*time.Millisecond {
			t.Errorf("initialBackoff=%v, want 500ms", rc.initialBackoff)
		}
		if rc.sleeper == nil {
			t.Error("sleeper not set")
		}
	})
	t.Run("nil cfg defaults", func(t *testing.T) {
		rc := retryConfigFromCfg(nil)
		if rc.timeout != 0 || rc.maxRetries != 0 || rc.initialBackoff != 0 {
			t.Errorf("nil cfg should yield all-zero retry config, got %+v", rc)
		}
	})
}
