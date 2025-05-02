package httputils

import (
	"net/http"
	"time"
)

// NewHTTPClient creates a new instance of a HTTP client
// configured with a timeout.
func NewHTTPClient(timeoutSeconds int) *http.Client {
	return &http.Client{
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
		Transport: http.DefaultTransport,
	}
}
