package handlers

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint-state/manage"
	"github.com/newstack-cloud/bluelink/libs/deploy-engine-client/types"
	"github.com/newstack-cloud/celerity/apps/cli/internal/testutils"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ValidateHandlerTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func TestValidateHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(ValidateHandlerTestSuite))
}

func (s *ValidateHandlerTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

func (s *ValidateHandlerTestSuite) Test_successful_validation_streams_events() {
	mockEngine := &testutils.MockDeployEngine{
		CreateBlueprintValidationResult: &manage.BlueprintValidation{
			ID: "val-123",
		},
		StubValidationEvents: []types.BlueprintValidationEvent{
			{ID: "evt-1"},
		},
	}

	var buf bytes.Buffer
	handler := NewValidateHandler(mockEngine, "app.blueprint.yaml", &buf, s.logger)

	err := handler.Handle(context.Background())
	s.Require().NoError(err)

	out := buf.String()
	s.Assert().Contains(out, "Validating blueprint file: app.blueprint.yaml")
	s.Assert().Contains(out, "Received event")
	s.Assert().Contains(out, "Validation stream closed")
}

func (s *ValidateHandlerTestSuite) Test_create_validation_error_propagates() {
	mockEngine := &testutils.MockDeployEngine{
		CreateBlueprintValidationErr: errors.New("connection refused"),
	}

	var buf bytes.Buffer
	handler := NewValidateHandler(mockEngine, "app.blueprint.yaml", &buf, s.logger)

	err := handler.Handle(context.Background())
	s.Assert().Error(err)
}

func (s *ValidateHandlerTestSuite) Test_stream_error_propagates() {
	mockEngine := &testutils.MockDeployEngine{
		CreateBlueprintValidationResult: &manage.BlueprintValidation{ID: "val-123"},
		StreamBlueprintValidationErr:    errors.New("stream failed"),
	}

	var buf bytes.Buffer
	handler := NewValidateHandler(mockEngine, "app.blueprint.yaml", &buf, s.logger)

	err := handler.Handle(context.Background())
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "stream failed")
}

func (s *ValidateHandlerTestSuite) Test_context_cancellation_returns_error() {
	mockEngine := &testutils.MockDeployEngine{
		CreateBlueprintValidationResult: &manage.BlueprintValidation{ID: "val-123"},
		StreamBlueprintValidationEventsFn: func(
			_ context.Context,
			_ string,
			streamTo chan<- types.BlueprintValidationEvent,
			_ chan<- error,
		) error {
			// Don't close the channel — let the context cancel
			return nil
		},
	}

	var buf bytes.Buffer
	handler := NewValidateHandler(mockEngine, "app.blueprint.yaml", &buf, s.logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := handler.Handle(ctx)
	s.Assert().Error(err)
	s.Assert().ErrorIs(err, context.Canceled)
}

func (s *ValidateHandlerTestSuite) Test_error_from_error_channel_propagates() {
	mockEngine := &testutils.MockDeployEngine{
		CreateBlueprintValidationResult: &manage.BlueprintValidation{ID: "val-123"},
		StreamBlueprintValidationEventsFn: func(
			_ context.Context,
			_ string,
			_ chan<- types.BlueprintValidationEvent,
			errChan chan<- error,
		) error {
			go func() {
				errChan <- errors.New("server error")
			}()
			return nil
		},
	}

	var buf bytes.Buffer
	handler := NewValidateHandler(mockEngine, "app.blueprint.yaml", &buf, s.logger)

	err := handler.Handle(context.Background())
	s.Assert().Error(err)
	s.Assert().Contains(err.Error(), "server error")
}
