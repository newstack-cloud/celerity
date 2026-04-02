package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type config struct {
	port     string
	issuer   string
	audience string
}

func loadConfig() config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9099"
	}

	issuer := os.Getenv("DEV_AUTH_ISSUER")
	if issuer == "" {
		issuer = "http://host.docker.internal:" + port
	}

	audience := os.Getenv("DEV_AUTH_AUDIENCE")
	if audience == "" {
		audience = "celerity-test-app"
	}

	return config{port: port, issuer: issuer, audience: audience}
}

type tokenRequest struct {
	Sub       string                 `json:"sub"`
	Claims    map[string]interface{} `json:"claims"`
	ExpiresIn string                 `json:"expiresIn"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   string `json:"expires_in"`
}

func Run(ctx context.Context, logger *zap.Logger) error {
	cfg := loadConfig()

	privateKey, err := generateKeyPair()
	if err != nil {
		return fmt.Errorf("generate key pair: %w", err)
	}

	publicJWK := publicKeyToJWK(&privateKey.PublicKey)
	jwksDoc := jwks{Keys: []jwk{publicJWK}}

	discoveryDoc := map[string]interface{}{
		"issuer":                                cfg.issuer,
		"jwks_uri":                              cfg.issuer + "/.well-known/jwks.json",
		"token_endpoint":                        cfg.issuer + "/token",
		"response_types_supported":              []string{"token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("oidc discovery request", zap.String("from", r.RemoteAddr))
		writeJSON(w, http.StatusOK, discoveryDoc)
	})

	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("jwks request", zap.String("from", r.RemoteAddr))
		writeJSON(w, http.StatusOK, jwksDoc)
	})

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		handleToken(w, r, privateKey, cfg, logger)
	})

	server := &http.Server{
		Addr:              "0.0.0.0:" + cfg.port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logger.Info("dev auth server starting",
		zap.String("port", cfg.port),
		zap.String("issuer", cfg.issuer),
		zap.String("audience", cfg.audience),
	)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}

	logger.Info("dev auth server stopped")
	return nil
}

func handleToken(w http.ResponseWriter, r *http.Request, privateKey *rsa.PrivateKey, cfg config, logger *zap.Logger) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	var req tokenRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Sub == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required field: sub"})
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn == "" {
		expiresIn = "1h"
	}

	duration, err := time.ParseDuration(expiresIn)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expiresIn duration"})
		return
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss": cfg.issuer,
		"aud": cfg.audience,
		"sub": req.Sub,
		"iat": now.Unix(),
		"exp": now.Add(duration).Unix(),
	}

	for k, v := range req.Claims {
		if k == "iss" || k == "aud" || k == "sub" || k == "iat" || k == "exp" {
			continue
		}
		claims[k] = v
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID

	signed, err := token.SignedString(privateKey)
	if err != nil {
		logger.Error("failed to sign token", zap.Error(err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to sign token"})
		return
	}

	logger.Info("token issued", zap.String("sub", req.Sub))
	writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
