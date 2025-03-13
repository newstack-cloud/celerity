package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/endpointsv1"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/types"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/utils"
)

func main() {
	apiVersion := utils.Getenv("CELERITY_API_VERSION", "v1")
	port := utils.Getenv("CELERITY_API_PORT", "8325")
	useUnixSocket := os.Getenv("CELERITY_API_USE_UNIX_SOCKET")
	unixSocketPath := utils.Getenv("CELERITY_API_UNIX_SOCKET_PATH", "/tmp/celerity.sock")
	// Fallback to loopback only as a more secure default.
	loopbackOnly := utils.Getenv("CELERITY_API_LOOPBACK_ONLY", "1")
	setup, setupExists := apiVersions[apiVersion]
	if !setupExists {
		log.Fatalf("version \"%s\" does not exist", apiVersion)
	}

	r := mux.NewRouter().PathPrefix(fmt.Sprintf("/%s", apiVersion)).Subrouter()
	_, err := setup(r)
	if err != nil {
		log.Fatalf("error setting up API: %s", err)
	}

	srv := &http.Server{
		Handler:      r,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	if useUnixSocket == "" || useUnixSocket == "0" || useUnixSocket == "false" {
		serverAddr := determineServerAddr(loopbackOnly, port)
		srv.Addr = serverAddr
		log.Fatal(srv.ListenAndServe())
	} else {
		startUnixSocketServer(srv, unixSocketPath)
	}
}

func startUnixSocketServer(srv *http.Server, unixSocketPath string) {
	listener, err := net.Listen("unix", unixSocketPath)
	if err != nil {
		log.Fatalf("error creating listener for unix socket: %s", err)
	}

	// Cleanup the sockfile.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Remove(unixSocketPath)
		os.Exit(1)
	}()

	defer listener.Close()
	log.Fatal(srv.Serve(listener))
}

func determineServerAddr(loopbackOnlyStr, portStr string) string {
	loopbackOnly, err := strconv.ParseBool(strings.TrimSpace(loopbackOnlyStr))
	if err != nil {
		log.Fatalf("error parsing loopback only value: \"%s\" is not boolean-like", err)
	}

	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil {
		log.Fatalf("error parsing port value: %s is not an integer", err)
	}

	if loopbackOnly {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}

	return fmt.Sprintf(":%d", port)
}

var apiVersions = map[string]types.SetupFunc{
	"v1": endpointsv1.Setup,
}
