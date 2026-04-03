package testutils

import (
	"net"
	"net/url"
	"os"
	"testing"
	"time"
)

// RequireEnv skips the test if the given environment variable is not set
// or the endpoint it points to is not reachable.
// Returns the value if present and reachable.
func RequireEnv(t *testing.T, envVar string) string {
	t.Helper()
	val := os.Getenv(envVar)
	if val == "" {
		t.Skipf("skipping: %s not set (start test infrastructure with docker-compose)", envVar)
	}

	// Probe the endpoint to skip gracefully when infra is down.
	host := extractHost(val)
	if host != "" {
		conn, err := net.DialTimeout("tcp", host, 1*time.Second)
		if err != nil {
			t.Skipf("skipping: %s=%s not reachable (start test infrastructure with docker-compose)", envVar, val)
		}
		conn.Close()
	}

	return val
}

// extractHost tries to parse a host:port from the value.
// Supports URLs (http://host:port), redis URLs (redis://host:port), and bare host:port.
func extractHost(val string) string {
	if u, err := url.Parse(val); err == nil && u.Host != "" {
		return u.Host
	}
	// Try bare host:port.
	if _, _, err := net.SplitHostPort(val); err == nil {
		return val
	}
	return ""
}
