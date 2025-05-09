package httputils

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30
	// DefaultHTTPRetryMax is the default maximum number of retries for HTTP requests.
	DefaultHTTPRetryMax = 10
)

// HTTPClientOption is a function that configures a http.Client.
type HTTPClientOption func(*http.Client)

// WithHTTPClientTimeout configures a http.Client instance
// with a timeout, where timeout is in seconds.
func WithHTTPClientTimeout(timeout int) HTTPClientOption {
	return func(c *http.Client) {
		c.Timeout = time.Second * time.Duration(timeout)
	}
}

// WithHTTPClientInstrumentation configures a http.Client instance
// with OpenTelemetry instrumentation.
func WithNativeHTTPClientInstrumentation() HTTPClientOption {
	return func(c *http.Client) {
		c.Transport = otelhttp.NewTransport(http.DefaultTransport)
	}
}

// WithHTTPClientRetryableTransport configures a http.Client instance
// with a retryable transport that support retries with exponential backoff.
func WithHTTPClientRetryableTransport(
	retryConfig *RetryConfig,
) HTTPClientOption {
	return func(c *http.Client) {
		prevTransport := c.Transport
		opts := []RetryableTransportOption{}
		if retryConfig != nil {
			opts = append(opts, WithTransportRetryConfig(retryConfig))
		}
		c.Transport = NewRetryableTransport(prevTransport, opts...)
	}
}

// NewHTTPClient creates a new instance of a HTTP client
// configured with a timeout.
func NewHTTPClient(opts ...HTTPClientOption) *http.Client {
	client := &http.Client{
		Timeout:   DefaultHTTPTimeout * time.Second,
		Transport: http.DefaultTransport,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}
