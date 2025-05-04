package testutils

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/links"
	"github.com/two-hundred/celerity/libs/blueprint/refgraph"
	"github.com/two-hundred/celerity/libs/blueprint/schema"
	"github.com/two-hundred/celerity/libs/blueprint/speccore"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	commoncore "github.com/two-hundred/celerity/libs/common/core"
)

type MockBlueprintLoader struct {
	stubDiagnostics            []*core.Diagnostic
	clock                      commoncore.Clock
	instances                  state.InstancesContainer
	deployEventSequence        []container.DeployEvent
	changeStagingEventSequence []ChangeStagingEvent
}

func NewMockBlueprintLoader(
	stubDiagnostics []*core.Diagnostic,
	clock commoncore.Clock,
	instances state.InstancesContainer,
	deployEventSequence []container.DeployEvent,
	changeStagingEventSequence []ChangeStagingEvent,
) container.Loader {
	return &MockBlueprintLoader{
		stubDiagnostics:            stubDiagnostics,
		clock:                      clock,
		instances:                  instances,
		deployEventSequence:        deployEventSequence,
		changeStagingEventSequence: changeStagingEventSequence,
	}
}

func (m *MockBlueprintLoader) Load(
	ctx context.Context,
	blueprintSpecFile string,
	params core.BlueprintParams,
) (container.BlueprintContainer, error) {
	return &MockBlueprintContainer{
		stubDiagnostics:            m.stubDiagnostics,
		clock:                      m.clock,
		instances:                  m.instances,
		deployEventSequence:        m.deployEventSequence,
		changeStagingEventSequence: m.changeStagingEventSequence,
	}, nil
}

func (m *MockBlueprintLoader) Validate(
	ctx context.Context,
	blueprintSpecFile string,
	params core.BlueprintParams,
) (*container.ValidationResult, error) {
	return &container.ValidationResult{
		Diagnostics: m.stubDiagnostics,
	}, nil
}

func (m *MockBlueprintLoader) LoadString(
	ctx context.Context,
	blueprintSpec string,
	inputFormat schema.SpecFormat,
	params core.BlueprintParams,
) (container.BlueprintContainer, error) {
	return &MockBlueprintContainer{
		stubDiagnostics:            m.stubDiagnostics,
		clock:                      m.clock,
		instances:                  m.instances,
		deployEventSequence:        m.deployEventSequence,
		changeStagingEventSequence: m.changeStagingEventSequence,
	}, nil
}

func (m *MockBlueprintLoader) ValidateString(
	ctx context.Context,
	blueprintSpec string,
	inputFormat schema.SpecFormat,
	params core.BlueprintParams,
) (*container.ValidationResult, error) {
	return &container.ValidationResult{
		Diagnostics: m.stubDiagnostics,
	}, nil
}

func (m *MockBlueprintLoader) LoadFromSchema(
	ctx context.Context,
	blueprintSchema *schema.Blueprint,
	params core.BlueprintParams,
) (container.BlueprintContainer, error) {
	return &MockBlueprintContainer{
		stubDiagnostics:            m.stubDiagnostics,
		clock:                      m.clock,
		instances:                  m.instances,
		deployEventSequence:        m.deployEventSequence,
		changeStagingEventSequence: m.changeStagingEventSequence,
	}, nil
}

func (m *MockBlueprintLoader) ValidateFromSchema(
	ctx context.Context,
	blueprintSchema *schema.Blueprint,
	params core.BlueprintParams,
) (*container.ValidationResult, error) {
	return &container.ValidationResult{
		Diagnostics: m.stubDiagnostics,
	}, nil
}

type MockBlueprintContainer struct {
	stubDiagnostics            []*core.Diagnostic
	clock                      commoncore.Clock
	instances                  state.InstancesContainer
	deployEventSequence        []container.DeployEvent
	changeStagingEventSequence []ChangeStagingEvent
}

func (m *MockBlueprintContainer) StageChanges(
	ctx context.Context,
	input *container.StageChangesInput,
	channels *container.ChangeStagingChannels,
	paramOverrides core.BlueprintParams,
) error {
	go func() {
		for _, event := range m.changeStagingEventSequence {
			if event.ResourceChangesEvent != nil {
				channels.ResourceChangesChan <- *event.ResourceChangesEvent
			}
			if event.ChildChangesEvent != nil {
				channels.ChildChangesChan <- *event.ChildChangesEvent
			}
			if event.LinkChangesEvent != nil {
				channels.LinkChangesChan <- *event.LinkChangesEvent
			}
			if event.FinalBlueprintChanges != nil {
				channels.CompleteChan <- *event.FinalBlueprintChanges
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return nil
}

func (m *MockBlueprintContainer) Deploy(
	ctx context.Context,
	input *container.DeployInput,
	channels *container.DeployChannels,
	paramOverrides core.BlueprintParams,
) error {
	instanceID := input.InstanceID
	if instanceID == "" {
		instanceID = uuid.New().String()
	}
	go func() {
		currentTimestamp := m.clock.Now().Unix()
		err := m.instances.Save(
			ctx,
			state.InstanceState{
				InstanceID:                instanceID,
				Status:                    core.InstanceStatusPreparing,
				LastStatusUpdateTimestamp: int(currentTimestamp),
			},
		)
		if err != nil {
			channels.ErrChan <- err
			return
		}

		for _, event := range m.deployEventSequence {
			if event.ResourceUpdateEvent != nil {
				event.ResourceUpdateEvent.InstanceID = instanceID
				channels.ResourceUpdateChan <- *event.ResourceUpdateEvent
			}
			if event.ChildUpdateEvent != nil {
				event.ChildUpdateEvent.ParentInstanceID = instanceID
				channels.ChildUpdateChan <- *event.ChildUpdateEvent
			}
			if event.LinkUpdateEvent != nil {
				event.LinkUpdateEvent.InstanceID = instanceID
				channels.LinkUpdateChan <- *event.LinkUpdateEvent
			}
			if event.DeploymentUpdateEvent != nil {
				event.DeploymentUpdateEvent.InstanceID = instanceID
				channels.DeploymentUpdateChan <- *event.DeploymentUpdateEvent
			}
			if event.FinishEvent != nil {
				event.FinishEvent.InstanceID = instanceID
				channels.FinishChan <- *event.FinishEvent
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	return nil
}

func (m *MockBlueprintContainer) Destroy(
	ctx context.Context,
	input *container.DestroyInput,
	channels *container.DeployChannels,
	paramOverrides core.BlueprintParams,
) {
	// Destroy doesn't need to do anything in the mock implementation.
}

func (m *MockBlueprintContainer) SpecLinkInfo() links.SpecLinkInfo {
	return nil
}

func (m *MockBlueprintContainer) BlueprintSpec() speccore.BlueprintSpec {
	return nil
}

func (m *MockBlueprintContainer) RefChainCollector() refgraph.RefChainCollector {
	return nil
}

func (m *MockBlueprintContainer) ResourceTemplates() map[string]string {
	return map[string]string{}
}

func (m *MockBlueprintContainer) Diagnostics() []*core.Diagnostic {
	return m.stubDiagnostics
}
