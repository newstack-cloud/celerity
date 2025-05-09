package httputils

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type HTTPClientSuite struct {
	suite.Suite
	client        *http.Client
	server        *httptest.Server
	retryAttempts int
}

func (s *HTTPClientSuite) SetupTest() {
	s.client = NewHTTPClient(
		WithHTTPClientTimeout(5),
		WithHTTPClientRetryableTransport(
			&RetryConfig{
				RetryCount:       5,
				MaxRetryDuration: 2 * time.Second,
				// 5 retries should take 1.5 seconds to complete with a backoff factor of 0.1
				// where T() = 0.1 * 2^retries.
				BackoffFactor: 0.1,
				// Use exponential backoff to make the test more predictable
				// than the default backoff with jitter.
				BackoffFunction:  ExponentialBackoff,
				RetryStatusCodes: []int{502},
				ShouldRetry:      DefaultShouldRetry,
			},
		),
	)
	s.server = s.createRetryServer()
}

func (s *HTTPClientSuite) TearDownTest() {
	s.server.Close()
}

func (s *HTTPClientSuite) Test_client_with_retry_backoff_behaviour() {
	req, err := http.NewRequest(http.MethodGet, s.server.URL, nil)
	s.Require().NoError(err)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	s.Assert().Equal(http.StatusOK, resp.StatusCode)
	s.Assert().Equal(5, s.retryAttempts)
}

func (s *HTTPClientSuite) createRetryServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.retryAttempts += 1
		if s.retryAttempts < 5 {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte{})
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte{})
	}))
}

func TestHTTPClientSuite(t *testing.T) {
	suite.Run(t, new(HTTPClientSuite))
}
