// The github.com/r3labs/sse/v2 package provides an interface
// for creating clients that takes options but doesn't implement
// any of the options. This file implements the options
// for the github.com/r3labs/sse/v2 package so it can be configured
// with the idiomatic With*Option pattern.
package sseconfig

import (
	"net/http"

	"github.com/r3labs/sse/v2"
)

// Option is a function that configures the sse.Client.
type Option func(*sse.Client)

// WithHeaders configures the sse.Client with the provided custom headers.
func WithHeaders(headers map[string]string) Option {
	return func(c *sse.Client) {
		c.Headers = headers
	}
}

// WithHTTPClient configures the http.Client to be used by the sse.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *sse.Client) {
		c.Connection = client
	}
}

// WithResponseValidator configures the sse.Client with the provided response validator.
// The response validator is used to validate the response from the server in order
// to handle non-200 status codes and other errors.
func WithResponseValidator(
	responseValidator sse.ResponseValidator,
) Option {
	return func(c *sse.Client) {
		c.ResponseValidator = responseValidator
	}
}
