package main

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/jwk"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

const (
	wellKnownDir   = "/public/.well-known"
	privateJWKPath = "/private/jwk_full.json"
	issuer         = "oidc-local-server"
	audience       = "oidc-local-client"
	subject        = "oidc-local-client"
	// The key ID in the private JSON Web Key (JWK) file
	// and the public JSON Web Key Set (JWKS) file.
	keyID = "test-key-1"
)

func main() {
	clientID := os.Getenv("OIDC_CLIENT_ID")
	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")

	privateKey, err := loadPrivateKey(privateJWKPath)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/.well-known/").Handler(
		http.StripPrefix("/.well-known/", http.FileServer(http.Dir(wellKnownDir))),
	)

	router.HandleFunc(
		"/oauth2/v1/token",
		basicAuth(tokenHandler(privateKey), clientID, clientSecret),
	).Methods("POST")

	server := &http.Server{
		Addr:         ":80",
		Handler:      router,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Starting server on port 80 ...")
	log.Fatal(server.ListenAndServe())
}

func loadPrivateKey(keyPath string) (jwk.Key, error) {
	privateKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return jwk.ParseKey(privateKeyData)
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

func tokenHandler(privateKey jwk.Key) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grantType := r.FormValue("grant_type")
		if grantType != "client_credentials" {
			http.Error(w, "unsupported grant type", http.StatusBadRequest)
			return
		}

		token, err := createToken(
			privateKey,
			subject,
			issuer,
			[]string{audience},
			keyID,
			map[string]any{},
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tokenRespBytes, err := json.Marshal(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(tokenRespBytes)
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func createToken(
	privateKey jwk.Key,
	subject string,
	issuer string,
	audience []string,
	kid string,
	customClaims map[string]any,
) (*tokenResponse, error) {
	privateKeyRaw := &rsa.PrivateKey{}
	err := privateKey.Raw(privateKeyRaw)
	if err != nil {
		return nil, nil
	}
	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       privateKeyRaw,
		},
		(&jose.SignerOptions{
			ExtraHeaders: map[jose.HeaderKey]any{
				"kid": kid,
			},
		}).WithType("JWT"),
	)
	if err != nil {
		return nil, err
	}
	claims := jwt.Claims{
		Subject:  subject,
		Issuer:   issuer,
		Audience: jwt.Audience(audience),
		// All the tokens issued by the local server are valid for 1 hour.
		Expiry: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}
	token, err := jwt.Signed(sig).Claims(claims).Claims(customClaims).CompactSerialize()
	if err != nil {
		return nil, err
	}

	return &tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		// The access token is valid for 1 hour.
		ExpiresIn: 3600,
	}, nil
}
