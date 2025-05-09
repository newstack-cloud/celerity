package testutils

import (
	"log"
	"net"
	"net/http"
	"net/http/httptest"
)

func NewUnixDomainSocketServer(
	socketPath string,
	handler http.Handler,
) *httptest.Server {
	server := httptest.NewUnstartedServer(handler)
	server.Listener.Close()
	server.Listener = newUnixDomainSocketListener(socketPath)
	server.Start()
	// We must set the URL to "http://unix" to prevent the client from
	// trying to resolve the address that will be a local file path
	// for the unix domain socket listener.
	server.URL = "http://unix"
	return server
}

func newUnixDomainSocketListener(socketPath string) net.Listener {
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	return listener
}
