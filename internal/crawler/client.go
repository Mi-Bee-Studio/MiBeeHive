package crawler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// MaxResponseBodySize is the maximum allowed HTTP response body size (100MB).
	// Responses larger than this are rejected to prevent OOM on the resource-constrained device.
	MaxResponseBodySize int64 = 100 * 1024 * 1024

	// ErrResponseBodyTooLarge is returned when an HTTP response body exceeds MaxResponseBodySize.
	ErrResponseBodyTooLarge = "response body too large: exceeds 100MB limit"
)

var (
	sharedClient     *http.Client
	sharedClientOnce sync.Once
)

// SharedHTTPClient returns a singleton HTTP client with connection pooling
// configured for all crawler HTTP requests. The client uses sensible defaults:
//   - 30s overall timeout
//   - Max 10 idle connections total, 5 per host
//   - 90s idle connection timeout
func SharedHTTPClient() *http.Client {
	sharedClientOnce.Do(func() {
		sharedClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return sharedClient
}

// LimitedReadAll reads the response body up to MaxResponseBodySize bytes.
// Returns an error if the body exceeds the limit.
func LimitedReadAll(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, MaxResponseBodySize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxResponseBodySize {
		return nil, errors.New(ErrResponseBodyTooLarge)
	}
	return data, nil
}

// LimitedReadBody reads an HTTP response body with size limiting and returns
// an error if the body exceeds MaxResponseBodySize. It also validates the
// Content-Length header when present.
func LimitedReadBody(resp *http.Response) ([]byte, error) {
	if resp.ContentLength > MaxResponseBodySize {
		return nil, fmt.Errorf("%s (Content-Length: %d bytes)", ErrResponseBodyTooLarge, resp.ContentLength)
	}
	return LimitedReadAll(resp.Body)
}
