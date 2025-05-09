package httputils

import (
	"bytes"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"
)

const (
	// DefaultRetryCount is the default number of times to retry a request.
	DefaultRetryCount int = 5
	// DefaultBackoffFactor is the default factor (or base) to use for exponential backoff.
	DefaultBackoffFactor float64 = 1.0
	// DefaultMaxRetryDuration is the default maximum duration allowed to wait before retrying a request.
	DefaultMaxRetryDuration time.Duration = 30 * time.Second
)

var (
	// DefaultRetryStatusCodes is a list of status codes that should be retried.
	// This includes 500, 502, 503, 504 and 429 status codes.
	DefaultRetryStatusCodes = []int{
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusInternalServerError,
		http.StatusTooManyRequests,
	}
)

// RetryConfig provides configuration options for retrying requests
// with exponential backoff.
type RetryConfig struct {
	// RetryCount is the number of times to retry a request.
	RetryCount int
	// BackoffFactor is the factor to use for exponential backoff.
	BackoffFactor float64
	// MaxRetryDuration is the maximum duration allowed to wait before retrying a request.
	MaxRetryDuration time.Duration
	// RetryStatusCodes is a list of HTTP status codes that should be retried.
	RetryStatusCodes []int
	// ShouldRetry is a function that determines if a request should be retried.
	ShouldRetry func(error, *http.Response, *RetryConfig) bool
	// BackoffFunction is a function that returns a time.Duration for exponential backoff.
	BackoffFunction func(int, *RetryConfig) time.Duration
}

// ExponentialBackoff returns a time.Duration for exponential backoff
// to be used in the retryable transport for a HTTP client.
func ExponentialBackoff(retries int, retryConfig *RetryConfig) time.Duration {
	candidateBackoff := retryConfig.BackoffFactor * math.Pow(2, float64(retries))
	backoff := math.Min(candidateBackoff, float64(retryConfig.MaxRetryDuration.Seconds()))
	return time.Duration(backoff) * time.Second
}

// ExponentialBackoffWithJitter returns a time.Duration for exponential backoff
// to be used in the retryable transport for a HTTP client.
func ExponentialBackoffWithJitter(retries int, retryConfig *RetryConfig) time.Duration {
	candidateBackoff := retryConfig.BackoffFactor * math.Pow(2, float64(retries))
	backoff := math.Min(candidateBackoff, float64(retryConfig.MaxRetryDuration.Seconds()))
	backoffInt := int(math.Floor(backoff))
	return time.Duration(rand.IntN(backoffInt)) * time.Second
}

// DefaultShouldRetry is a function that determines if a request should be retried.
// This will retry on network errors and the configured status codes.
func DefaultShouldRetry(err error, resp *http.Response, retryConfig *RetryConfig) bool {
	if err != nil {
		return true
	}

	return resp != nil && slices.Contains(retryConfig.RetryStatusCodes, resp.StatusCode)
}

func drainBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// RetryableTransport is a transport that retries requests based on the configuration.
type RetryableTransport struct {
	transport   http.RoundTripper
	retryConfig *RetryConfig
}

// RetryableTransportOption is a function that configures a RetryableTransport.
type RetryableTransportOption func(*RetryableTransport)

// WithTransportRetryConfig is an option that sets the
// retry configuration for a RetryableTransport.
func WithTransportRetryConfig(
	provider *RetryConfig,
) RetryableTransportOption {
	return func(t *RetryableTransport) {
		t.retryConfig = provider
	}
}

// NewRetryableTransport creates a new RetryableTransport with the provided underlying
// transport and options.
func NewRetryableTransport(
	transport http.RoundTripper,
	opts ...RetryableTransportOption,
) *RetryableTransport {
	retryable := &RetryableTransport{
		transport: transport,
	}

	for _, opt := range opts {
		opt(retryable)
	}

	if retryable.retryConfig == nil {
		retryable.retryConfig = createDefaultRetryConfig()
	}

	return retryable
}

// RoundTrip executes a single HTTP transaction and returns a Response for the provided Request
// with retries based on the configuration.
func (t *RetryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request body so it can be read multiple times.
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	resp, err := t.transport.RoundTrip(req)
	retries := 0

	for t.retryConfig.ShouldRetry(err, resp, t.retryConfig) && retries < t.retryConfig.RetryCount {
		time.Sleep(t.retryConfig.BackoffFunction(retries, t.retryConfig))
		// Consume any response to reuse the connection for the next retry.
		drainBody(resp)

		// Clone the request body again so it can be used in the next retry.
		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err = t.transport.RoundTrip(req)
		retries += 1
	}

	return resp, err
}

func createDefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		RetryCount:       DefaultRetryCount,
		BackoffFactor:    DefaultBackoffFactor,
		MaxRetryDuration: DefaultMaxRetryDuration,
		RetryStatusCodes: DefaultRetryStatusCodes,
		ShouldRetry:      DefaultShouldRetry,
		BackoffFunction:  ExponentialBackoffWithJitter,
	}
}
