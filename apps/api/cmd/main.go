package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/apps/api/internal/endpointsv1"
	"github.com/two-hundred/celerity/apps/api/internal/types"
	"github.com/two-hundred/celerity/apps/api/internal/utils"
)

func main() {
	apiVersion := utils.Getenv("CELERITY_API_VERSION", "v1")
	port := utils.Getenv("CELERITY_API_PORT", "8325")
	// Fallback to loopback only as a more secure default.
	loopbackOnly := utils.Getenv("CELERITY_LOOPBACK_ONLY", "1")
	setup, setupExists := apiVersions[apiVersion]
	if !setupExists {
		log.Fatalf("version \"%s\" does not exist", apiVersion)
	}

	serverAddr := determineServerAddr(loopbackOnly, port)

	r := mux.NewRouter().PathPrefix(fmt.Sprintf("/%s", apiVersion)).Subrouter()
	_, err := setup(r)
	if err != nil {
		log.Fatalf("error setting up API: %s", err)
	}

	srv := &http.Server{
		Handler:      r,
		Addr:         serverAddr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
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
