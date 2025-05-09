package deployengine

func getTokenEndpoint(
	config *OAuth2Config,
) string {
	if config == nil {
		return ""
	}

	return config.TokenEndpoint
}

func getProviderBaseURL(
	config *OAuth2Config,
) string {
	if config == nil {
		return ""
	}

	return config.ProviderBaseURL
}

func getClientID(
	config *OAuth2Config,
) string {
	if config == nil {
		return ""
	}

	return config.ClientID
}

func getClientSecret(
	config *OAuth2Config,
) string {
	if config == nil {
		return ""
	}

	return config.ClientSecret
}
