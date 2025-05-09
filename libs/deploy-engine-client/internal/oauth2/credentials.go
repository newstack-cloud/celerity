package oauth2

import (
	"context"
	"net/http"

	oauth2ext "golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// CredentialsHelper is an interface for a service that can retrieve OAuth2 access
// tokens to interact with OAuth2-protected APIs.
type CredentialsHelper interface {
	// GetAccessToken retrieves an access token for the given OAuth2 client.
	GetAccessToken() (string, error)
}

type credentialsHelperImpl struct {
	oauthConfig *clientcredentials.Config
	client      *http.Client
	tokenSource oauth2ext.TokenSource
	// A static context that last the lifetime of the service instance
	// to be used for token retrieval.
	staticCtx context.Context
}

// NewCredentialsHelper creates a new instance of a service that can retrieve
// OAuth2 access tokens.
func NewCredentialsHelper(
	oauthConfig *clientcredentials.Config,
	client *http.Client,
	staticCtx context.Context,
) CredentialsHelper {
	return &credentialsHelperImpl{
		oauthConfig: oauthConfig,
		client:      client,
		staticCtx:   staticCtx,
	}
}

func (h *credentialsHelperImpl) GetAccessToken() (string, error) {
	if h.tokenSource == nil {
		err := h.fetchInitialAccessToken()
		if err != nil {
			return "", err
		}
	}
	token, err := h.tokenSource.Token()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (h *credentialsHelperImpl) fetchInitialAccessToken() error {
	ctxWithClient := context.WithValue(h.staticCtx, oauth2ext.HTTPClient, h.client)
	h.tokenSource = h.oauthConfig.TokenSource(ctxWithClient)
	return nil
}
