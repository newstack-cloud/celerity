package devtest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/devrun"
	"github.com/stretchr/testify/suite"
)

type HealthTestSuite struct {
	suite.Suite
}

func TestHealthTestSuite(t *testing.T) {
	suite.Run(t, new(HealthTestSuite))
}

func (s *HealthTestSuite) Test_healthy_server_returns_immediately() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	output := devrun.NewOutput(&buf, false)

	err := WaitForHealth(context.Background(), server.URL, "/health", 5*time.Second, output)
	s.Require().NoError(err)
	s.Assert().Contains(buf.String(), "App ready")
}

func (s *HealthTestSuite) Test_uses_default_health_path_when_empty() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == DefaultHealthPath {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	var buf bytes.Buffer
	output := devrun.NewOutput(&buf, false)

	err := WaitForHealth(context.Background(), server.URL, "", 5*time.Second, output)
	s.Require().NoError(err)
}

func (s *HealthTestSuite) Test_retries_until_healthy() {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	output := devrun.NewOutput(&buf, false)

	err := WaitForHealth(context.Background(), server.URL, "/health", 10*time.Second, output)
	s.Require().NoError(err)
	s.Assert().GreaterOrEqual(attempts, 3)
}

func (s *HealthTestSuite) Test_timeout_returns_error() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	var buf bytes.Buffer
	output := devrun.NewOutput(&buf, false)

	err := WaitForHealth(context.Background(), server.URL, "/health", 1*time.Second, output)
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "did not become healthy")
}

func (s *HealthTestSuite) Test_context_cancellation_returns_error() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	var buf bytes.Buffer
	output := devrun.NewOutput(&buf, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WaitForHealth(ctx, server.URL, "/health", 30*time.Second, output)
	s.Assert().Error(err)
}
