package testutils

import (
	"net/http"
	"net/http/httptest"
)

func CreateFailingServer() *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
}
