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
	"github.com/two-hundred/celerity/libs/blueprint/core"
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

func TestControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ControllerTestSuite))
}
