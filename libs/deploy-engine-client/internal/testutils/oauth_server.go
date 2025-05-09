package testutils

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

func CreateOAuthServer(
	clientID string, clientSecret string, authType string,
) (*httptest.Server, error) {
	router := mux.NewRouter()
	router.HandleFunc(
		"/oauth2/v1/token",
		basicAuth(tokenHandler, clientID, clientSecret),
	).Methods("POST")

	discoveryDocument := determineDiscoveryDocument(authType)

	metadata, err := os.ReadFile(
		fmt.Sprintf("__testdata/%s", discoveryDocument),
	)
	if err != nil {
		return nil, err
	}

	var serverURL string

	discoveryEndpoint := fmt.Sprintf("/.well-known/%s", discoveryDocument)
	router.HandleFunc(discoveryEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(replaceServerURL(metadata, serverURL))
	})

	server := httptest.NewServer(router)
	serverURL = server.URL

	return server, nil
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"access_token":"test-token-1","token_type":"bearer","expires_in":3600}`))
}

func basicAuth(
	next http.HandlerFunc,
	expectedUsername string,
	expectedPassword string,
) http.HandlerFunc {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if ok {
				// Calculate the SHA-256 hashes for the provided and expected
				// credentials.
				usernameHash := sha256.Sum256([]byte(username))
				passwordHash := sha256.Sum256([]byte(password))
				expectedUsernameHash := sha256.Sum256([]byte(expectedUsername))
				expectedPasswordHash := sha256.Sum256([]byte(expectedPassword))

				// Use subtle.ConstantTimeCompare to check if the provided
				// username and password hashes are equal.
				// ConstantTimeCompare is a constant-time comparison function
				// that helps prevent timing attacks.
				// It will return 1 if the values are equal, and 0 otherwise.
				usernameMatches := subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1
				passwordMatches := subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1

				if usernameMatches && passwordMatches {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		},
	)
}

func determineDiscoveryDocument(authType string) string {
	if authType == "oidc" {
		return "openid-configuration"
	}
	return "oauth-authorization-server"
}

func replaceServerURL(data []byte, serverURL string) []byte {
	return []byte(strings.ReplaceAll(string(data), "{server}", serverURL))
}
