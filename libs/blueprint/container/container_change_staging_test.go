package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

const (
	blueprint1InstanceID = "blueprint-instance-1"
)

type ContainerChangeStagingTestSuite struct {
	blueprint1Container BlueprintContainer
	suite.Suite
}

func (s *ContainerChangeStagingTestSuite) SetupSuite() {
	stateContainer := internal.NewMemoryStateContainer()
	resourceChangeStager := NewDefaultResourceChangeStager()
	err := s.populateCurrentState(stateContainer)
	s.Require().NoError(err)

	providers := map[string]provider.Provider{
		"aws":     newTestAWSProvider(),
		"example": newTestExampleProvider(),
		"core": providerhelpers.NewCoreProvider(
			stateContainer,
			core.BlueprintInstanceIDFromContext,
			os.Getwd,
			core.SystemClock{},
		),
	}
	specTransformers := map[string]transform.SpecTransformer{}
	loader := NewDefaultLoader(
		providers,
		specTransformers,
		stateContainer,
		resourceChangeStager,
		newFSChildResolver(),
		validation.NewRefChainCollector,
		WithLoaderTransformSpec(false),
		WithLoaderValidateRuntimeValues(true),
	)
	blueprint1Container, err := loader.Load(
		context.Background(),
		"__testdata/container/change-staging/blueprint1.yml",
		createBlueprint1Params(),
	)
	s.Require().NoError(err)
	s.blueprint1Container = blueprint1Container
}

func (s *ContainerChangeStagingTestSuite) Test_stage_changes_to_existing_blueprint_instance() {
	channels := createChangeStagingChannels()
	params := createBlueprint1Params()

	err := s.blueprint1Container.StageChanges(
		context.Background(),
		blueprint1InstanceID,
		channels,
		params,
	)
	s.Require().NoError(err)

	resourceChangeMessages := []ResourceChangesMessage{}
	childChangeMessages := []ChildChangesMessage{}
	linkChangeMessages := []LinkChangesMessage{}
	fullChangeSet := (*BlueprintChanges)(nil)
	for err == nil &&
		(fullChangeSet == nil ||
			len(resourceChangeMessages) < 6 ||
			len(childChangeMessages) < 1 ||
			len(linkChangeMessages) < 3) {
		select {
		case msg := <-channels.ResourceChangesChan:
			resourceChangeMessages = append(resourceChangeMessages, msg)
		case msg := <-channels.ChildChangesChan:
			childChangeMessages = append(childChangeMessages, msg)
		case msg := <-channels.LinkChangesChan:
			linkChangeMessages = append(linkChangeMessages, msg)
		case changeSet := <-channels.CompleteChan:
			fullChangeSet = &changeSet
		case err = <-channels.ErrChan:
		case <-time.After(60 * time.Second):
			err = fmt.Errorf("timed out waiting for changes to be staged")
		}
	}
	s.Require().NoError(err)

	err = cupaloy.Snapshot(normaliseBlueprintChanges(fullChangeSet))
	s.Require().NoError(err)
}

func (s *ContainerChangeStagingTestSuite) Test_stage_changes_for_a_new_blueprint_instance() {
}

func (s *ContainerChangeStagingTestSuite) populateCurrentState(stateContainer state.Container) error {

	blueprint1CurrentState, err := s.loadCurrentState(
		"__testdata/container/change-staging/current-state/blueprint1.json",
	)
	if err != nil {
		return err
	}
	err = stateContainer.SaveInstance(
		context.Background(),
		*blueprint1CurrentState,
	)
	if err != nil {
		return err
	}

	blueprint1ChildCurrentState, err := s.loadCurrentState(
		"__testdata/container/change-staging/current-state/blueprint1-child-core-infra.json",
	)
	if err != nil {
		return err
	}

	return stateContainer.SaveChild(
		context.Background(),
		blueprint1InstanceID,
		"coreInfra",
		*blueprint1ChildCurrentState,
	)
}

func (s *ContainerChangeStagingTestSuite) loadCurrentState(
	stateSnapshotFile string,
) (*state.InstanceState, error) {
	currentStateBytes, err := os.ReadFile(stateSnapshotFile)
	if err != nil {
		return nil, err
	}

	currentState := &state.InstanceState{}
	err = json.Unmarshal(currentStateBytes, currentState)
	if err != nil {
		return nil, err
	}

	return currentState, nil
}

func createBlueprint1Params() *internal.Params {
	environment := "production-env"
	enableOrderTableTrigger := true
	region := "us-west-2"
	deployOrdersTableToRegions := "[\"us-west-2\",\"us-east-1\"]"
	relatedInfo := "[{\"id\":\"test-info-1\"},{\"id\":\"test-info-2\"}]"
	includeInvoices := false
	orderTablesConfig := "[{\"name\":\"orders-1\"},{\"name\":\"orders-2\"}]"
	blueprintVars := map[string]*core.ScalarValue{
		"environment": {
			StringValue: &environment,
		},
		"region": {
			StringValue: &region,
		},
		"deployOrdersTableToRegions": {
			StringValue: &deployOrdersTableToRegions,
		},
		"enableOrderTableTrigger": {
			BoolValue: &enableOrderTableTrigger,
		},
		"relatedInfo": {
			StringValue: &relatedInfo,
		},
		"includeInvoices": {
			BoolValue: &includeInvoices,
		},
		"orderTablesConfig": {
			StringValue: &orderTablesConfig,
		},
	}
	return internal.NewParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		blueprintVars,
	)
}

func TestContainerChangesStagingTestSuite(t *testing.T) {
	suite.Run(t, new(ContainerChangeStagingTestSuite))
}
