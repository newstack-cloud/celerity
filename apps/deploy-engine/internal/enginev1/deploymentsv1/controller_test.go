package deploymentsv1

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/enginev1/helpersv1"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/enginev1/typesv1"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/params"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/testutils"
	"github.com/two-hundred/celerity/apps/deploy-engine/utils"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/changes"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

var (
	// Saturday, 3 May 2025 14:27:22 UTC
	testTime = time.Unix(1746282442, 0).UTC()
)

type ControllerTestSuite struct {
	suite.Suite
	ctrl                    *Controller
	ctrlFailingIDGenerators *Controller
	eventStore              manage.Events
	changesetStore          manage.Changesets
	instances               state.InstancesContainer
	client                  *http.Client
}

func (s *ControllerTestSuite) SetupTest() {
	stateContainer := testutils.NewMemoryStateContainer()
	clock := &testutils.MockClock{
		StaticTime: testTime,
	}
	blueprintLoader := testutils.NewMockBlueprintLoader(
		[]*core.Diagnostic{},
		clock,
		stateContainer.Instances(),
		// Leave instance ID empty for the deploy event sequence
		// so it can be populated based on the instance ID provided
		// in the request or generated in the deployment process.
		deployEventSequence( /* instanceID */ ""),
		changeStagingEventSequence(),
	)
	s.eventStore = testutils.NewMockEventStore(
		map[string]*manage.Event{},
	)
	s.changesetStore = testutils.NewMockChangesetStore(
		map[string]*manage.Changeset{},
	)
	s.instances = stateContainer.Instances()
	dependencies := &typesv1.Dependencies{
		EventStore: s.eventStore,
		ValidationStore: testutils.NewMockBlueprintValidationStore(
			map[string]*manage.BlueprintValidation{},
		),
		ChangesetStore:    s.changesetStore,
		Instances:         s.instances,
		Exports:           stateContainer.Exports(),
		IDGenerator:       core.NewUUIDGenerator(),
		EventIDGenerator:  utils.NewUUIDv7Generator(),
		ValidationLoader:  blueprintLoader,
		DeploymentLoader:  blueprintLoader,
		BlueprintResolver: &testutils.MockBlueprintResolver{},
		ParamsProvider: params.NewDefaultProvider(
			map[string]*core.ScalarValue{},
		),
		Clock:  clock,
		Logger: core.NewNopLogger(),
	}
	s.ctrl = NewController(
		/* changesetRetentionPeriod */ 10*time.Second,
		/* deploymentTimeout */ 10*time.Second,
		dependencies,
	)
	depsWithFailingIDGenerators := testutils.CopyDependencies(dependencies)
	failingIDGenerator := &testutils.FailingIDGenerator{}
	depsWithFailingIDGenerators.IDGenerator = failingIDGenerator
	depsWithFailingIDGenerators.EventIDGenerator = failingIDGenerator
	s.ctrlFailingIDGenerators = NewController(
		/* changesetRetentionPeriod */ 10*time.Second,
		/* deploymentTimeout */ 10*time.Second,
		depsWithFailingIDGenerators,
	)
	s.client = &http.Client{
		Timeout: 10 * time.Second,
	}

	helpersv1.SetupRequestBodyValidator()
}

func deployEventSequence(instanceID string) []container.DeployEvent {
	return []container.DeployEvent{
		{
			DeploymentUpdateEvent: &container.DeploymentUpdateMessage{
				InstanceID:      instanceID,
				Status:          core.InstanceStatusPreparing,
				UpdateTimestamp: testTime.Unix(),
			},
		},
		{
			ResourceUpdateEvent: &container.ResourceDeployUpdateMessage{
				InstanceID:      instanceID,
				ResourceID:      "resource-1",
				ResourceName:    "Resource1",
				Status:          core.ResourceStatusCreating,
				PreciseStatus:   core.PreciseResourceStatusCreating,
				UpdateTimestamp: testTime.Unix(),
			},
		},
		{
			LinkUpdateEvent: &container.LinkDeployUpdateMessage{
				InstanceID:      instanceID,
				LinkID:          "link-1",
				LinkName:        "Resource1::Resource2",
				Status:          core.LinkStatusCreating,
				PreciseStatus:   core.PreciseLinkStatusUpdatingResourceA,
				UpdateTimestamp: testTime.Unix(),
			},
		},
		{
			ChildUpdateEvent: &container.ChildDeployUpdateMessage{
				ParentInstanceID: instanceID,
				ChildInstanceID:  "child-instance-1",
				ChildName:        "coreInfra",
				Status:           core.InstanceStatusDeploying,
				UpdateTimestamp:  testTime.Unix(),
			},
		},
		{
			DeploymentUpdateEvent: &container.DeploymentUpdateMessage{
				InstanceID:      instanceID,
				Status:          core.InstanceStatusDeploying,
				UpdateTimestamp: testTime.Unix(),
			},
		},
		{
			FinishEvent: &container.DeploymentFinishedMessage{
				InstanceID:      instanceID,
				Status:          core.InstanceStatusDeployed,
				UpdateTimestamp: testTime.Unix(),
			},
		},
	}
}

func changeStagingEventSequence() []testutils.ChangeStagingEvent {
	return []testutils.ChangeStagingEvent{
		{
			ResourceChangesEvent: &container.ResourceChangesMessage{
				ResourceName: "Resource1",
				Removed:      true,
			},
		},
		{
			ChildChangesEvent: &container.ChildChangesMessage{
				ChildBlueprintName: "coreInfra",
				New:                true,
				Changes: changes.BlueprintChanges{
					NewResources: map[string]provider.Changes{
						"childResource1": {
							ComputedFields: []string{"spec.id"},
						},
					},
				},
			},
		},
		{
			LinkChangesEvent: &container.LinkChangesMessage{
				ResourceAName: "Resource1",
				ResourceBName: "Resource2",
				Changes: provider.LinkChanges{
					FieldChangesKnownOnDeploy: []string{"Resource1.policy"},
				},
			},
		},
		{
			FinalBlueprintChanges: &changes.BlueprintChanges{
				ResourceChanges: map[string]provider.Changes{
					"Resource1": {
						NewOutboundLinks: map[string]provider.LinkChanges{
							"Resource2": {
								FieldChangesKnownOnDeploy: []string{"Resource1.policy"},
							},
						},
					},
				},
				NewChildren: map[string]changes.NewBlueprintDefinition{
					"coreInfra": {
						NewResources: map[string]provider.Changes{
							"childResource1": {
								ComputedFields: []string{"spec.id"},
							},
						},
					},
				},
			},
		},
	}
}

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
