package devtest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
)

const (
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 5 * time.Second

	// DefaultHealthPath is the built-in health endpoint exposed by the Celerity
	// runtime. It is always registered unless the developer sets
	// CELERITY_USE_CUSTOM_HEALTH_CHECK=true in the runtime environment.
	DefaultHealthPath = "/runtime/health/check"
)

// WaitForHealth polls the given base URL + health path until the server responds
// with a 2xx status code or the timeout expires.
//
// healthPath overrides the endpoint to poll. When empty, DefaultHealthPath is
// used. Developers who set CELERITY_USE_CUSTOM_HEALTH_CHECK=true can pass their
// custom path via --health-path.
func WaitForHealth(
	ctx context.Context,
	baseURL string,
	healthPath string,
	timeout time.Duration,
	output *devrun.Output,
) error {
	if healthPath == "" {
		healthPath = DefaultHealthPath
	}
	url := baseURL + healthPath

	output.PrintHealthWaiting()
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	backoff := initialBackoff
	client := &http.Client{Timeout: 5 * time.Second}

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("creating health check request: %w", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				output.PrintHealthReady(time.Since(start))
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"app did not become healthy within %s; use 'celerity dev logs' to diagnose",
				timeout,
			)
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
