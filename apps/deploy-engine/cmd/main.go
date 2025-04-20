package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/apps/deploy-engine/core"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/enginev1"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/types"
)

func main() {
	config, err := core.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("error loading config: %s", err)
	}

	apiVersion := config.Version
	port := config.Port
	useUnixSocket := config.UseUnixSocket
	unixSocketPath := config.UnixSocketPath
	loopbackOnly := config.LoopbackOnly
	setup, setupExists := apiVersions[apiVersion]
	if !setupExists {
		log.Fatalf("version \"%s\" does not exist", apiVersion)
	}

	r := mux.NewRouter().PathPrefix(fmt.Sprintf("/%s", apiVersion)).Subrouter()
	_, err = setup(r, &config)
	if err != nil {
		log.Fatalf("error setting up Deploy Engine API: %s", err)
	}

	srv := &http.Server{
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           http.TimeoutHandler(r, 60*time.Second, "Timeout\n"),
	}

	if useUnixSocket {
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

func determineServerAddr(loopbackOnly bool, port int) string {
	if loopbackOnly {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}

	return fmt.Sprintf(":%d", port)
}

var apiVersions = map[string]types.SetupFunc{
	"v1": enginev1.Setup,
}
