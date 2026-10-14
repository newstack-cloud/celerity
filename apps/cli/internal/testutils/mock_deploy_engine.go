package testutils

import (
	"context"
	"time"

	"github.com/newstack-cloud/bluelink/libs/blueprint-state/manage"
	"github.com/newstack-cloud/bluelink/libs/blueprint/container"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/bluelink/libs/deploy-engine-client/types"
)

// MockDeployEngine is a test double for engine.DeployEngine.
// Set the fields to control return values and inject errors.
//
// The create and destroy calls answer with a response wrapper carrying a
// LastEventID alongside the resource, because that is what the engine returns
// and what a caller has to pass on to the matching stream call. The *Result
// fields hold the resource, and the wrapper is built around it, so a test
// setting one does not have to know the wrapper exists.
type MockDeployEngine struct {
	CreateBlueprintValidationResult *manage.BlueprintValidation
	CreateBlueprintValidationErr    error

	GetBlueprintValidationResult *manage.BlueprintValidation
	GetBlueprintValidationErr    error

	// StreamBlueprintValidationEventsFn allows full control over the streaming behaviour.
	// If set, it is called directly; otherwise the default implementation
	// sends StubValidationEvents to the streamTo channel and then closes it.
	StreamBlueprintValidationEventsFn func(ctx context.Context, validationID string, lastEventID string, streamTo chan<- types.BlueprintValidationEvent, errChan chan<- error) error
	StreamBlueprintValidationErr      error
	StubValidationEvents              []types.BlueprintValidationEvent

	CreateChangesetResult *manage.Changeset
	CreateChangesetErr    error

	GetChangesetResult *manage.Changeset
	GetChangesetErr    error

	StreamChangeStagingEventsFn func(ctx context.Context, changesetID string, lastEventID string, streamTo chan<- types.ChangeStagingEvent, errChan chan<- error) error
	StreamChangeStagingErr      error

	CreateBlueprintInstanceResult *state.InstanceState
	CreateBlueprintInstanceErr    error

	UpdateBlueprintInstanceResult *state.InstanceState
	UpdateBlueprintInstanceErr    error

	GetBlueprintInstanceResult *state.InstanceState
	GetBlueprintInstanceErr    error

	ListBlueprintInstancesResult state.ListInstancesResult
	ListBlueprintInstancesErr    error

	GetBlueprintInstanceExportsResult map[string]*state.ExportState
	GetBlueprintInstanceExportsErr    error

	DestroyBlueprintInstanceResult *state.InstanceState
	DestroyBlueprintInstanceErr    error

	StreamBlueprintInstanceEventsFn func(ctx context.Context, instanceID string, lastEventID string, streamTo chan<- types.BlueprintInstanceEvent, errChan chan<- error) error
	StreamBlueprintInstanceErr      error

	// LastEventID is returned on every create and destroy response, so a test
	// can assert that it reaches the stream call that should carry it.
	LastEventID string

	CleanupOperationResult *manage.CleanupOperation
	CleanupErr             error

	ApplyReconciliationResult *container.ApplyReconciliationResult
	ApplyReconciliationErr    error
}

func (m *MockDeployEngine) CreateBlueprintValidation(
	_ context.Context,
	_ *types.CreateBlueprintValidationPayload,
	_ *types.CreateBlueprintValidationQuery,
) (*types.BlueprintValidationResponse, error) {
	if m.CreateBlueprintValidationErr != nil {
		return nil, m.CreateBlueprintValidationErr
	}
	return &types.BlueprintValidationResponse{
		LastEventID: m.LastEventID,
		Data:        m.CreateBlueprintValidationResult,
	}, nil
}

func (m *MockDeployEngine) GetBlueprintValidation(_ context.Context, _ string) (*manage.BlueprintValidation, error) {
	return m.GetBlueprintValidationResult, m.GetBlueprintValidationErr
}

func (m *MockDeployEngine) StreamBlueprintValidationEvents(
	ctx context.Context,
	validationID string,
	lastEventID string,
	streamTo chan<- types.BlueprintValidationEvent,
	errChan chan<- error,
) error {
	if m.StreamBlueprintValidationEventsFn != nil {
		return m.StreamBlueprintValidationEventsFn(ctx, validationID, lastEventID, streamTo, errChan)
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

func (m *MockDeployEngine) CleanupBlueprintValidations(_ context.Context) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) CreateChangeset(
	_ context.Context,
	_ *types.CreateChangesetPayload,
) (*types.ChangesetResponse, error) {
	if m.CreateChangesetErr != nil {
		return nil, m.CreateChangesetErr
	}
	return &types.ChangesetResponse{
		LastEventID: m.LastEventID,
		Data:        m.CreateChangesetResult,
	}, nil
}

func (m *MockDeployEngine) GetChangeset(_ context.Context, _ string) (*manage.Changeset, error) {
	return m.GetChangesetResult, m.GetChangesetErr
}

func (m *MockDeployEngine) StreamChangeStagingEvents(
	ctx context.Context,
	changesetID string,
	lastEventID string,
	streamTo chan<- types.ChangeStagingEvent,
	errChan chan<- error,
) error {
	if m.StreamChangeStagingEventsFn != nil {
		return m.StreamChangeStagingEventsFn(ctx, changesetID, lastEventID, streamTo, errChan)
	}
	return m.StreamChangeStagingErr
}

func (m *MockDeployEngine) CleanupChangesets(_ context.Context) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) CreateBlueprintInstance(
	_ context.Context,
	_ *types.BlueprintInstancePayload,
) (*types.BlueprintInstanceResponse, error) {
	if m.CreateBlueprintInstanceErr != nil {
		return nil, m.CreateBlueprintInstanceErr
	}
	return m.instanceResponse(m.CreateBlueprintInstanceResult), nil
}

func (m *MockDeployEngine) UpdateBlueprintInstance(
	_ context.Context,
	_ string,
	_ *types.BlueprintInstancePayload,
) (*types.BlueprintInstanceResponse, error) {
	if m.UpdateBlueprintInstanceErr != nil {
		return nil, m.UpdateBlueprintInstanceErr
	}
	return m.instanceResponse(m.UpdateBlueprintInstanceResult), nil
}

func (m *MockDeployEngine) GetBlueprintInstance(_ context.Context, _ string) (*state.InstanceState, error) {
	return m.GetBlueprintInstanceResult, m.GetBlueprintInstanceErr
}

func (m *MockDeployEngine) ListBlueprintInstances(
	_ context.Context,
	_ state.ListInstancesParams,
) (state.ListInstancesResult, error) {
	return m.ListBlueprintInstancesResult, m.ListBlueprintInstancesErr
}

func (m *MockDeployEngine) GetBlueprintInstanceExports(_ context.Context, _ string) (map[string]*state.ExportState, error) {
	return m.GetBlueprintInstanceExportsResult, m.GetBlueprintInstanceExportsErr
}

func (m *MockDeployEngine) DestroyBlueprintInstance(
	_ context.Context,
	_ string,
	_ *types.DestroyBlueprintInstancePayload,
) (*types.BlueprintInstanceResponse, error) {
	if m.DestroyBlueprintInstanceErr != nil {
		return nil, m.DestroyBlueprintInstanceErr
	}
	return m.instanceResponse(m.DestroyBlueprintInstanceResult), nil
}

func (m *MockDeployEngine) StreamBlueprintInstanceEvents(
	ctx context.Context,
	instanceID string,
	lastEventID string,
	streamTo chan<- types.BlueprintInstanceEvent,
	errChan chan<- error,
) error {
	if m.StreamBlueprintInstanceEventsFn != nil {
		return m.StreamBlueprintInstanceEventsFn(ctx, instanceID, lastEventID, streamTo, errChan)
	}
	return m.StreamBlueprintInstanceErr
}

func (m *MockDeployEngine) CleanupEvents(_ context.Context) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) CleanupReconciliationResults(_ context.Context) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) GetCleanupOperation(
	_ context.Context,
	_ manage.CleanupType,
	_ string,
) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) WaitForCleanupCompletion(
	_ context.Context,
	_ manage.CleanupType,
	_ string,
	_ time.Duration,
) (*manage.CleanupOperation, error) {
	return m.CleanupOperationResult, m.CleanupErr
}

func (m *MockDeployEngine) ApplyReconciliation(
	_ context.Context,
	_ string,
	_ *types.ApplyReconciliationPayload,
) (*container.ApplyReconciliationResult, error) {
	return m.ApplyReconciliationResult, m.ApplyReconciliationErr
}

// instanceResponse wraps an instance in the response shape the engine returns.
//
// Data is a value rather than a pointer there, so a nil result -- which is what
// a test that only cares about the error leaves behind -- becomes the zero
// instance rather than a panic.
func (m *MockDeployEngine) instanceResponse(instance *state.InstanceState) *types.BlueprintInstanceResponse {
	response := &types.BlueprintInstanceResponse{LastEventID: m.LastEventID}
	if instance != nil {
		response.Data = *instance
	}
	return response
}
