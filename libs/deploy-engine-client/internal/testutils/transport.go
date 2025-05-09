package testutils

import "net/http"

// CreateDefaultTransport provides a function that overrides
// the default transport configured for a deploy engine client
// with the Go default HTTP transport.
func CreateDefaultTransport(transport *http.Transport) http.RoundTripper {
	return http.DefaultTransport
}
