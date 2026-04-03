package testutils

import (
	"context"

	"github.com/newstack-cloud/bluelink/libs/blueprint-state/manage"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/bluelink/libs/deploy-engine-client/types"
)

// MockDeployEngine is a test double for engine.DeployEngine.
// Set the fields to control return values and inject errors.
type MockDeployEngine struct {
	CreateBlueprintValidationResult *manage.BlueprintValidation
	CreateBlueprintValidationErr    error

	GetBlueprintValidationResult *manage.BlueprintValidation
	GetBlueprintValidationErr    error

	// StreamBlueprintValidationEventsFn allows full control over the streaming behaviour.
	// If set, it is called directly; otherwise the default implementation
	// sends StubValidationEvents to the streamTo channel and then closes it.
	StreamBlueprintValidationEventsFn func(ctx context.Context, validationID string, streamTo chan<- types.BlueprintValidationEvent, errChan chan<- error) error
	StreamBlueprintValidationErr      error
	StubValidationEvents              []types.BlueprintValidationEvent

	CreateChangesetResult *manage.Changeset
	CreateChangesetErr    error

	GetChangesetResult *manage.Changeset
	GetChangesetErr    error

	StreamChangeStagingEventsFn func(ctx context.Context, changesetID string, streamTo chan<- types.ChangeStagingEvent, errChan chan<- error) error
	StreamChangeStagingErr      error

	CreateBlueprintInstanceResult *state.InstanceState
	CreateBlueprintInstanceErr    error

	UpdateBlueprintInstanceResult *state.InstanceState
	UpdateBlueprintInstanceErr    error

	GetBlueprintInstanceResult *state.InstanceState
	GetBlueprintInstanceErr    error

	GetBlueprintInstanceExportsResult map[string]*state.ExportState
	GetBlueprintInstanceExportsErr    error

	DestroyBlueprintInstanceResult *state.InstanceState
	DestroyBlueprintInstanceErr    error

	StreamBlueprintInstanceEventsFn func(ctx context.Context, instanceID string, streamTo chan<- types.BlueprintInstanceEvent, errChan chan<- error) error
	StreamBlueprintInstanceErr      error
}

func (m *MockDeployEngine) CreateBlueprintValidation(
	_ context.Context,
	_ *types.CreateBlueprintValidationPayload,
	_ *types.CreateBlueprintValidationQuery,
) (*manage.BlueprintValidation, error) {
	return m.CreateBlueprintValidationResult, m.CreateBlueprintValidationErr
}

func (m *MockDeployEngine) GetBlueprintValidation(_ context.Context, _ string) (*manage.BlueprintValidation, error) {
	return m.GetBlueprintValidationResult, m.GetBlueprintValidationErr
}

func (m *MockDeployEngine) StreamBlueprintValidationEvents(
	ctx context.Context,
	validationID string,
	streamTo chan<- types.BlueprintValidationEvent,
	errChan chan<- error,
) error {
	if m.StreamBlueprintValidationEventsFn != nil {
		return m.StreamBlueprintValidationEventsFn(ctx, validationID, streamTo, errChan)
	}
	if m.StreamBlueprintValidationErr != nil {
		return m.StreamBlueprintValidationErr
	}
	go func() {
		for _, e := range m.StubValidationEvents {
			streamTo <- e
		}
		close(streamTo)
	}()
	return nil
}

func (m *MockDeployEngine) CleanupBlueprintValidations(_ context.Context) error {
	return nil
}

func (m *MockDeployEngine) CreateChangeset(_ context.Context, _ *types.CreateChangesetPayload) (*manage.Changeset, error) {
	return m.CreateChangesetResult, m.CreateChangesetErr
}

func (m *MockDeployEngine) GetChangeset(_ context.Context, _ string) (*manage.Changeset, error) {
	return m.GetChangesetResult, m.GetChangesetErr
}

func (m *MockDeployEngine) StreamChangeStagingEvents(
	ctx context.Context,
	changesetID string,
	streamTo chan<- types.ChangeStagingEvent,
	errChan chan<- error,
) error {
	if m.StreamChangeStagingEventsFn != nil {
		return m.StreamChangeStagingEventsFn(ctx, changesetID, streamTo, errChan)
	}
	return m.StreamChangeStagingErr
}

func (m *MockDeployEngine) CleanupChangesets(_ context.Context) error {
	return nil
}

func (m *MockDeployEngine) CreateBlueprintInstance(_ context.Context, _ *types.BlueprintInstancePayload) (*state.InstanceState, error) {
	return m.CreateBlueprintInstanceResult, m.CreateBlueprintInstanceErr
}

func (m *MockDeployEngine) UpdateBlueprintInstance(_ context.Context, _ string, _ *types.BlueprintInstancePayload) (*state.InstanceState, error) {
	return m.UpdateBlueprintInstanceResult, m.UpdateBlueprintInstanceErr
}

func (m *MockDeployEngine) GetBlueprintInstance(_ context.Context, _ string) (*state.InstanceState, error) {
	return m.GetBlueprintInstanceResult, m.GetBlueprintInstanceErr
}

func (m *MockDeployEngine) GetBlueprintInstanceExports(_ context.Context, _ string) (map[string]*state.ExportState, error) {
	return m.GetBlueprintInstanceExportsResult, m.GetBlueprintInstanceExportsErr
}

func (m *MockDeployEngine) DestroyBlueprintInstance(_ context.Context, _ string, _ *types.DestroyBlueprintInstancePayload) (*state.InstanceState, error) {
	return m.DestroyBlueprintInstanceResult, m.DestroyBlueprintInstanceErr
}

func (m *MockDeployEngine) StreamBlueprintInstanceEvents(
	ctx context.Context,
	instanceID string,
	streamTo chan<- types.BlueprintInstanceEvent,
	errChan chan<- error,
) error {
	if m.StreamBlueprintInstanceEventsFn != nil {
		return m.StreamBlueprintInstanceEventsFn(ctx, instanceID, streamTo, errChan)
	}
	return m.StreamBlueprintInstanceErr
}

func (m *MockDeployEngine) CleanupEvents(_ context.Context) error {
	return nil
}
