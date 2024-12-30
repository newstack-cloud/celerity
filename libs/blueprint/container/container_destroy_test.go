package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	"github.com/two-hundred/celerity/libs/blueprint/providerhelpers"
	"github.com/two-hundred/celerity/libs/blueprint/state"
	"github.com/two-hundred/celerity/libs/blueprint/transform"
	"github.com/two-hundred/celerity/libs/blueprint/validation"
)

type ContainerDestroyTestSuite struct {
	blueprint1Fixture blueprintDeployFixture
	blueprint2Fixture blueprintDeployFixture
	blueprint3Fixture blueprintDeployFixture
	stateContainer    state.Container
	suite.Suite
}

func (s *ContainerDestroyTestSuite) SetupTest() {
	stateContainer := internal.NewMemoryStateContainer()
	s.stateContainer = stateContainer
	resourceChangeStager := NewDefaultResourceChangeStager()
	err := s.populateCurrentState(stateContainer)
	s.Require().NoError(err)

	providers := map[string]provider.Provider{
		"aws":     newTestAWSProvider(),
		"example": newTestExampleProvider(),
		"core": providerhelpers.NewCoreProvider(
			stateContainer.Links(),
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
		"__testdata/container/destroy/blueprint1.yml",
		baseBlueprintParams(),
	)
	s.Require().NoError(err)
	s.blueprint1Fixture, err = createBlueprintDeployFixture(
		blueprint1Container,
		"__testdata/container/destroy/expected-messages/blueprint1.json",
	)
	s.Require().NoError(err)

	blueprint2Container, err := loader.Load(
		context.Background(),
		"__testdata/container/destroy/blueprint2.yml",
		baseBlueprintParams(),
	)
	s.Require().NoError(err)
	s.blueprint2Fixture, err = createBlueprintDeployFixture(
		blueprint2Container,
		"__testdata/container/destroy/expected-messages/blueprint2.json",
	)
	s.Require().NoError(err)

	blueprint3Container, err := loader.Load(
		context.Background(),
		"__testdata/container/destroy/blueprint3.yml",
		baseBlueprintParams(),
	)
	s.Require().NoError(err)
	s.blueprint3Fixture, err = createBlueprintDeployFixture(
		blueprint3Container,
		"__testdata/container/destroy/expected-messages/blueprint3.json",
	)
	s.Require().NoError(err)
}

func (s *ContainerDestroyTestSuite) Test_destroys_blueprint_instance_with_child_blueprint() {
	channels := CreateDeployChannels()
	s.blueprint1Fixture.blueprintContainer.Destroy(
		context.Background(),
		&DestroyInput{
			InstanceID: "blueprint-instance-1",
			Changes:    blueprint1RemovalChanges(),
			Rollback:   false,
		},
		channels,
		blueprintDestroyParams(),
	)

	resourceUpdateMessages := []ResourceDeployUpdateMessage{}
	childDeployUpdateMessages := []ChildDeployUpdateMessage{}
	linkDeployUpdateMessages := []LinkDeployUpdateMessage{}
	deploymentUpdateMessages := []DeploymentUpdateMessage{}
	finishedMessage := (*DeploymentFinishedMessage)(nil)
	var err error
	for err == nil &&
		finishedMessage == nil {
		select {
		case msg := <-channels.ResourceUpdateChan:
			resourceUpdateMessages = append(resourceUpdateMessages, msg)
		case msg := <-channels.ChildUpdateChan:
			childDeployUpdateMessages = append(childDeployUpdateMessages, msg)
		case msg := <-channels.LinkUpdateChan:
			linkDeployUpdateMessages = append(linkDeployUpdateMessages, msg)
		case msg := <-channels.FinishChan:
			finishedMessage = &msg
		case msg := <-channels.DeploymentUpdateChan:
			deploymentUpdateMessages = append(deploymentUpdateMessages, msg)
		case err = <-channels.ErrChan:
		case <-time.After(60 * time.Second):
			err = errors.New(timeoutMessage)
		}
	}
	s.Require().NoError(err)

	actualMessages := &actualMessages{
		resourceDeployUpdateMessages: resourceUpdateMessages,
		childDeployUpdateMessages:    childDeployUpdateMessages,
		linkDeployUpdateMessages:     linkDeployUpdateMessages,
		deploymentUpdateMessages:     deploymentUpdateMessages,
		finishedMessage:              finishedMessage,
	}
	assertDeployMessageOrder(actualMessages, s.blueprint1Fixture.expected, &s.Suite)

	_, err = s.stateContainer.Instances().Get(context.Background(), "blueprint-instance-1")
	s.Assert().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *ContainerDestroyTestSuite) Test_destroys_blueprint_instance_as_deployment_rollback() {
	channels := CreateDeployChannels()
	s.blueprint2Fixture.blueprintContainer.Destroy(
		context.Background(),
		&DestroyInput{
			InstanceID: "blueprint-instance-2",
			Changes:    blueprint2RemovalChanges(),
			Rollback:   true,
		},
		channels,
		blueprintDestroyParams(),
	)

	resourceUpdateMessages := []ResourceDeployUpdateMessage{}
	childDeployUpdateMessages := []ChildDeployUpdateMessage{}
	linkDeployUpdateMessages := []LinkDeployUpdateMessage{}
	deploymentUpdateMessages := []DeploymentUpdateMessage{}
	finishedMessage := (*DeploymentFinishedMessage)(nil)
	var err error
	for err == nil &&
		finishedMessage == nil {
		select {
		case msg := <-channels.ResourceUpdateChan:
			resourceUpdateMessages = append(resourceUpdateMessages, msg)
		case msg := <-channels.ChildUpdateChan:
			childDeployUpdateMessages = append(childDeployUpdateMessages, msg)
		case msg := <-channels.LinkUpdateChan:
			linkDeployUpdateMessages = append(linkDeployUpdateMessages, msg)
		case msg := <-channels.FinishChan:
			finishedMessage = &msg
		case msg := <-channels.DeploymentUpdateChan:
			deploymentUpdateMessages = append(deploymentUpdateMessages, msg)
		case err = <-channels.ErrChan:
		case <-time.After(60 * time.Second):
			err = errors.New(timeoutMessage)
		}
	}
	s.Require().NoError(err)

	actualMessages := &actualMessages{
		resourceDeployUpdateMessages: resourceUpdateMessages,
		childDeployUpdateMessages:    childDeployUpdateMessages,
		linkDeployUpdateMessages:     linkDeployUpdateMessages,
		deploymentUpdateMessages:     deploymentUpdateMessages,
		finishedMessage:              finishedMessage,
	}
	assertDeployMessageOrder(actualMessages, s.blueprint2Fixture.expected, &s.Suite)

	_, err = s.stateContainer.Instances().Get(context.Background(), "blueprint-instance-2")
	s.Assert().Error(err)
	stateErr, isStateErr := err.(*state.Error)
	s.Assert().True(isStateErr)
	s.Assert().Equal(state.ErrInstanceNotFound, stateErr.Code)
}

func (s *ContainerDestroyTestSuite) Test_fails_to_destroys_blueprint_instance_due_to_terminal_resource_impl_error() {
	channels := CreateDeployChannels()
	s.blueprint3Fixture.blueprintContainer.Destroy(
		context.Background(),
		&DestroyInput{
			InstanceID: "blueprint-instance-3",
			Changes:    blueprint3RemovalChanges(),
			Rollback:   false,
		},
		channels,
		blueprintDestroyParams(),
	)

	resourceUpdateMessages := []ResourceDeployUpdateMessage{}
	finishedMessage := (*DeploymentFinishedMessage)(nil)
	var err error
	for err == nil &&
		finishedMessage == nil {
		select {
		case msg := <-channels.ResourceUpdateChan:
			resourceUpdateMessages = append(resourceUpdateMessages, msg)
		case <-channels.ChildUpdateChan:
		case <-channels.LinkUpdateChan:
		case msg := <-channels.FinishChan:
			finishedMessage = &msg
		case <-channels.DeploymentUpdateChan:
		case err = <-channels.ErrChan:
		case <-time.After(60 * time.Second):
			err = errors.New(timeoutMessage)
		}
	}
	s.Require().NoError(err)

	actualMessages := &actualMessages{
		resourceDeployUpdateMessages: resourceUpdateMessages,
		childDeployUpdateMessages:    []ChildDeployUpdateMessage{},
		linkDeployUpdateMessages:     []LinkDeployUpdateMessage{},
		deploymentUpdateMessages:     []DeploymentUpdateMessage{},
		finishedMessage:              finishedMessage,
	}
	assertDeployMessageOrder(actualMessages, s.blueprint3Fixture.expected, &s.Suite)

	instance, err := s.stateContainer.Instances().Get(context.Background(), "blueprint-instance-3")
	s.Assert().NoError(err)
	s.Assert().Equal("blueprint-instance-3", instance.InstanceID)
}

func (s *ContainerDestroyTestSuite) populateCurrentState(stateContainer state.Container) error {
	err := s.populateBlueprintCurrentState(stateContainer, "blueprint-instance-1", 1)
	if err != nil {
		return err
	}

	err = s.populateBlueprintCurrentState(stateContainer, "blueprint-instance-2", 2)
	if err != nil {
		return err
	}

	return s.populateBlueprintCurrentState(stateContainer, "blueprint-instance-3", 3)
}

func (s *ContainerDestroyTestSuite) populateBlueprintCurrentState(
	stateContainer state.Container,
	instanceID string,
	blueprintNo int,
) error {
	blueprintCurrentState, err := internal.LoadInstanceState(
		fmt.Sprintf(
			"__testdata/container/destroy/current-state/blueprint%d.json",
			blueprintNo,
		),
	)
	if err != nil {
		return err
	}
	err = stateContainer.Instances().Save(
		context.Background(),
		*blueprintCurrentState,
	)
	if err != nil {
		return err
	}

	blueprintChildCurrentState, err := internal.LoadInstanceState(
		fmt.Sprintf(
			"__testdata/container/destroy/current-state/blueprint%d-child-core-infra.json",
			blueprintNo,
		),
	)
	if err != nil {
		return err
	}

	return stateContainer.Children().Save(
		context.Background(),
		instanceID,
		"coreInfra",
		*blueprintChildCurrentState,
	)
}

func blueprint1RemovalChanges() *BlueprintChanges {
	return &BlueprintChanges{
		RemovedResources: []string{
			"ordersTable_0",
			"ordersTable_1",
			"saveOrderFunction",
			"invoicesTable",
		},
		RemovedChildren: []string{
			"coreInfra",
		},
		RemovedLinks: []string{
			"saveOrderFunction::ordersTable_0",
			"saveOrderFunction::ordersTable_1",
		},
		RemovedExports: []string{
			"environment",
		},
		ChildChanges: map[string]BlueprintChanges{
			"coreInfra": {
				RemovedResources: []string{
					"complexResource",
				},
				RemovedChildren: []string{},
				RemovedLinks:    []string{},
				RemovedExports:  []string{},
			},
		},
	}
}

func blueprint2RemovalChanges() *BlueprintChanges {
	return &BlueprintChanges{
		RemovedResources: []string{
			"ordersTable_0",
			"ordersTable_1",
			"preprocessOrderFunction",
			"invoicesTable",
		},
		RemovedChildren: []string{
			"coreInfra",
		},
		RemovedLinks: []string{
			"preprocessOrderFunction::ordersTable_0",
			"preprocessOrderFunction::ordersTable_1",
		},
		RemovedExports: []string{
			"environment",
		},
		ChildChanges: map[string]BlueprintChanges{
			"coreInfra": {
				RemovedResources: []string{
					"complexResource",
				},
				RemovedChildren: []string{},
				RemovedLinks:    []string{},
				RemovedExports:  []string{},
			},
		},
	}
}

func blueprint3RemovalChanges() *BlueprintChanges {
	return &BlueprintChanges{
		RemovedResources: []string{
			"ordersTable_0",
			"ordersTable_1",
			"failingOrderFunction",
			"invoicesTable",
		},
		RemovedChildren: []string{
			"coreInfra",
		},
		RemovedLinks: []string{
			"failingOrderFunction::ordersTable_0",
			"failingOrderFunction::ordersTable_1",
		},
		RemovedExports: []string{
			"environment",
		},
		ChildChanges: map[string]BlueprintChanges{
			"coreInfra": {
				RemovedResources: []string{
					"complexResource",
				},
				RemovedChildren: []string{},
				RemovedLinks:    []string{},
				RemovedExports:  []string{},
			},
		},
	}
}

func blueprintDestroyParams() core.BlueprintParams {
	return core.NewDefaultParams(
		map[string]map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
		map[string]*core.ScalarValue{},
	)
}

func TestContainerDestroyTestSuite(t *testing.T) {
	suite.Run(t, new(ContainerDestroyTestSuite))
}
