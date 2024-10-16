package languageservices

import (
	"github.com/two-hundred/ls-builder/common"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
	"go.uber.org/zap"
)

// SettingsService is a service that provides shared functionality
// for working with settings across multiple documents.
type SettingsService struct {
	state         *State
	configSection string
	logger        *zap.Logger
}

// NewSettingsService creates a new service for managing settings.
func NewSettingsService(state *State, configSection string, logger *zap.Logger) *SettingsService {
	return &SettingsService{
		state:         state,
		configSection: configSection,
		logger:        logger,
	}
}

// GetDocumentSettings retrieves the settings for a document
// from the server cache if possible, otherwise it requests
// the settings from the client.
func (s *SettingsService) GetDocumentSettings(
	context *common.LSPContext,
	uri string,
) (*DocSettings, error) {
	settings := s.state.GetDocumentSettings(uri)

	if settings != nil {
		return settings, nil
	} else {
		dispatcher := lsp.NewDispatcher(context)
		configResponse := []DocSettings{}
		err := dispatcher.WorkspaceConfiguration(
			lsp.ConfigurationParams{
				Items: []lsp.ConfigurationItem{
					{
						ScopeURI: &uri,
						Section:  &s.configSection,
					},
				},
			},
			&configResponse,
		)
		if err != nil {
			s.logger.Error("Failed to get document settings", zap.Error(err))
			return nil, err
		}

		err = context.Notify(
			"window/logMessage",
			&lsp.LogMessageParams{
				Type:    lsp.MessageTypeInfo,
				Message: "document workspace configuration (server received)",
			})
		if err != nil {
			return nil, err
		}

		if len(configResponse) > 0 {
			s.state.SetDocumentSettings(uri, &configResponse[0])
			return &configResponse[0], nil
		}
	}

	return &DocSettings{
		Trace: DocTraceSettings{
			Server: "off",
		},
		MaxNumberOfProblems: 100,
	}, nil
}
