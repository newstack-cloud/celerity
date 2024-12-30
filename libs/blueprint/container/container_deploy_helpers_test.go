package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func assertDeployMessageOrder(
	actual *actualMessages,
	expected *expectedMessages,
	testSuite *suite.Suite,
) {
	assertResourceUpdateMessageOrder(
		actual.resourceDeployUpdateMessages,
		expected.ResourceDeployUpdateMessages,
		testSuite,
	)
	assertChildUpdateMessageOrder(
		actual.childDeployUpdateMessages,
		expected.ChildDeployUpdateMessages,
		testSuite,
	)
	assertLinkUpdateMessageOrder(
		actual.linkDeployUpdateMessages,
		expected.LinkDeployUpdateMessages,
		testSuite,
	)
	assertDeploymentUpdateMessageOrder(
		actual.deploymentUpdateMessages,
		expected.DeploymentUpdateMessages,
		testSuite,
	)
	assertFinishedMessage(
		*actual.finishedMessage,
		*expected.FinishedMessage,
		testSuite,
	)
}

// Assert that the order of messages for each resource from a deployment
// are in the expected order.
//
// This will only compare the following fields:
// - InstanceID
// - ResourceID
// - ResourceName
// - Group
// - Status
// - PreciseStatus
// - FailureReasons
// - Attempt
// - CanRetry
// - UpdateTimestamp (Only checks for presence of timestamp)
// - Durations (Only checks for presence of duration fields)
//
// For all numeric fields where the value will not be compared,
// the value should be set to -1 in the expected messages.
func assertResourceUpdateMessageOrder(
	messages []ResourceDeployUpdateMessage,
	// Each element in the top-level slice represents the expected order of messages for a
	// a given resource.
	expectedPerResource [][]ResourceDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for _, expectedSequence := range expectedPerResource {
		if len(expectedSequence) > 0 {
			messagesForResource := getMessagesForResource(messages, expectedSequence[0].ResourceName)
			testSuite.Assert().Len(messagesForResource, len(expectedSequence))
			assertResourceMessagesEqual(messagesForResource, expectedSequence, testSuite)
		}
	}
}

func getMessagesForResource(
	messages []ResourceDeployUpdateMessage,
	resourceName string,
) []ResourceDeployUpdateMessage {
	resourceMessages := []ResourceDeployUpdateMessage{}
	for _, message := range messages {
		if message.ResourceName == resourceName {
			resourceMessages = append(resourceMessages, message)
		}
	}
	return resourceMessages
}

func assertResourceMessagesEqual(
	messages []ResourceDeployUpdateMessage,
	expectedMessages []ResourceDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for i, message := range messages {
		expected := expectedMessages[i]
		testSuite.Assert().Equal(expected.InstanceID, message.InstanceID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.ResourceID, message.ResourceID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.ResourceName, message.ResourceName, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.Group, message.Group, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.Status, message.Status, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.PreciseStatus, message.PreciseStatus, "actual message: %+v", message)
		assertFailureReasonsEqual(expected.FailureReasons, message.FailureReasons, testSuite)
		testSuite.Assert().Equal(expected.Attempt, message.Attempt, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.CanRetry, message.CanRetry, "actual message: %+v", message)
		assertTimestampPresent(expected.UpdateTimestamp, message.UpdateTimestamp, testSuite)
		assertResourceMessageDurations(expected.Durations, message.Durations, testSuite)
	}
}

func assertResourceMessageDurations(
	expectedDurations *state.ResourceCompletionDurations,
	actualDurations *state.ResourceCompletionDurations,
	testSuite *suite.Suite,
) {
	if expectedDurations != nil {
		testSuite.Assert().NotNil(actualDurations)
		if expectedDurations.TotalDuration != nil {
			testSuite.Assert().NotNil(actualDurations.TotalDuration)
		}

		if expectedDurations.ConfigCompleteDuration != nil {
			testSuite.Assert().NotNil(actualDurations.ConfigCompleteDuration)
		}

		if expectedDurations.AttemptDurations != nil {
			assertAttemptDurationsPresent(
				expectedDurations.AttemptDurations,
				actualDurations.AttemptDurations,
				testSuite,
			)
		}

	}
}

// Assert that the order of messages for each child blueprint from a deployment
// are in the expected order.
//
// This will only compare the following fields:
// - ParentInstanceID
// - ChildInstanceID
// - ChildName
// - Group
// - Status
// - FailureReasons
// - UpdateTimestamp (Only checks for presence of timestamp)
// - Durations (Only checks for presence of duration fields)
//
// For all numeric fields where the value will not be compared,
// the value should be set to -1 in the expected messages.
func assertChildUpdateMessageOrder(
	messages []ChildDeployUpdateMessage,
	// Each element in the top-level slice represents the expected order of messages for a
	// a given child blueprint.
	expectedPerChild [][]ChildDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for _, expectedSequence := range expectedPerChild {
		if len(expectedSequence) > 0 {
			messagesForChild := getMessagesForChild(messages, expectedSequence[0].ChildName)
			testSuite.Assert().Len(messagesForChild, len(expectedSequence))
			assertChildMessagesEqual(messagesForChild, expectedSequence, testSuite)
		}
	}
}

func getMessagesForChild(
	messages []ChildDeployUpdateMessage,
	childName string,
) []ChildDeployUpdateMessage {
	childMessages := []ChildDeployUpdateMessage{}
	for _, message := range messages {
		if message.ChildName == childName {
			childMessages = append(childMessages, message)
		}
	}
	return childMessages
}

func assertChildMessagesEqual(
	messages []ChildDeployUpdateMessage,
	expectedMessages []ChildDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for i, message := range messages {
		expected := expectedMessages[i]
		testSuite.Assert().Equal(expected.ParentInstanceID, message.ParentInstanceID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.ChildInstanceID, message.ChildInstanceID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.ChildName, message.ChildName, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.Group, message.Group, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.Status, message.Status, "actual message: %+v", message)
		assertFailureReasonsEqual(expected.FailureReasons, message.FailureReasons, testSuite)
		assertTimestampPresent(expected.UpdateTimestamp, message.UpdateTimestamp, testSuite)
		assertChildMessageDurations(expected.Durations, message.Durations, testSuite)
	}
}

func assertChildMessageDurations(
	expectedDurations *state.InstanceCompletionDuration,
	actualDurations *state.InstanceCompletionDuration,
	testSuite *suite.Suite,
) {
	if expectedDurations != nil {
		testSuite.Assert().NotNil(actualDurations)
		if expectedDurations.TotalDuration != nil {
			testSuite.Assert().NotNil(actualDurations.TotalDuration)
		}

		if expectedDurations.PrepareDuration != nil {
			testSuite.Assert().NotNil(actualDurations.PrepareDuration)
		}
	}
}

// Assert that the order of messages for each link from a deployment
// are in the expected order.
//
// This will only compare the following fields:
// - InstanceID
// - LinkID
// - LinkName
// - Status
// - PreciseStatus
// - CurrentStageAttempt
// - CanRetryCurrentStage
// - FailureReasons
// - UpdateTimestamp (Only checks for presence of timestamp)
// - Durations (Only checks for presence of duration fields)
//
// For all numeric fields where the value will not be compared,
// the value should be set to -1 in the expected messages.
func assertLinkUpdateMessageOrder(
	messages []LinkDeployUpdateMessage,
	// Each element in the top-level slice represents the expected order of messages for a
	// a given link.
	expectedPerLink [][]LinkDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for _, expectedSequence := range expectedPerLink {
		if len(expectedSequence) > 0 {
			messagesForLink := getMessagesForLink(messages, expectedSequence[0].LinkName)
			testSuite.Assert().Len(messagesForLink, len(expectedSequence))
			assertLinkMessagesEqual(messagesForLink, expectedSequence, testSuite)
		}
	}
}

func getMessagesForLink(
	messages []LinkDeployUpdateMessage,
	linkName string,
) []LinkDeployUpdateMessage {
	linkMessages := []LinkDeployUpdateMessage{}
	for _, message := range messages {
		if message.LinkName == linkName {
			linkMessages = append(linkMessages, message)
		}
	}
	return linkMessages
}

func assertLinkMessagesEqual(
	messages []LinkDeployUpdateMessage,
	expectedMessages []LinkDeployUpdateMessage,
	testSuite *suite.Suite,
) {
	for i, message := range messages {
		expected := expectedMessages[i]
		testSuite.Assert().Equal(expected.InstanceID, message.InstanceID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.LinkID, message.LinkID, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.LinkName, message.LinkName, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.Status, message.Status, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.PreciseStatus, message.PreciseStatus, "actual message: %+v", message)
		assertFailureReasonsEqual(expected.FailureReasons, message.FailureReasons, testSuite)
		testSuite.Assert().Equal(expected.CurrentStageAttempt, message.CurrentStageAttempt, "actual message: %+v", message)
		testSuite.Assert().Equal(expected.CanRetryCurrentStage, message.CanRetryCurrentStage, "actual message: %+v", message)
		assertTimestampPresent(expected.UpdateTimestamp, message.UpdateTimestamp, testSuite)
		assertLinkMessageDurations(expected.Durations, message.Durations, testSuite)
	}
}

func assertLinkMessageDurations(
	expectedDurations *state.LinkCompletionDurations,
	actualDurations *state.LinkCompletionDurations,
	testSuite *suite.Suite,
) {
	if expectedDurations != nil {
		testSuite.Assert().NotNil(actualDurations)
		if expectedDurations.TotalDuration != nil {
			testSuite.Assert().NotNil(actualDurations.TotalDuration)
		}

		if expectedDurations.ResourceAUpdate != nil {
			assertLinkComponentAttemptDurationsPresent(
				expectedDurations.ResourceAUpdate,
				actualDurations.ResourceAUpdate,
				testSuite,
			)
		}

		if expectedDurations.ResourceBUpdate != nil {
			assertLinkComponentAttemptDurationsPresent(
				expectedDurations.ResourceBUpdate,
				actualDurations.ResourceBUpdate,
				testSuite,
			)
		}

		if expectedDurations.IntermediaryResources != nil {
			assertLinkComponentAttemptDurationsPresent(
				expectedDurations.IntermediaryResources,
				actualDurations.IntermediaryResources,
				testSuite,
			)
		}
	}
}

func assertLinkComponentAttemptDurationsPresent(
	expectedDurations *state.LinkComponentCompletionDurations,
	actualDurations *state.LinkComponentCompletionDurations,
	testSuite *suite.Suite,
) {
	if expectedDurations != nil {
		testSuite.Assert().NotNil(actualDurations)
		if expectedDurations.TotalDuration != nil {
			testSuite.Assert().NotNil(actualDurations.TotalDuration)
		}

		if expectedDurations.AttemptDurations != nil {
			assertAttemptDurationsPresent(
				expectedDurations.AttemptDurations,
				actualDurations.AttemptDurations,
				testSuite,
			)
		}
	}
}

// Assert that the order of messages for the top-level deployment
// are in the expected order.
//
// This will compare the following fields:
// - InstanceID
// - Status
// - UpdateTimestamp (Only checks for presence of timestamp)
//
// For all numeric fields where the value will not be compared,
// the value should be set to -1 in the expected messages.
func assertDeploymentUpdateMessageOrder(
	messages []DeploymentUpdateMessage,
	// Each element in the top-level slice represents the expected order of messages for a
	// a deployment.
	expectedPerDeployment [][]DeploymentUpdateMessage,
	testSuite *suite.Suite,
) {
	for _, expectedSequence := range expectedPerDeployment {
		if len(expectedSequence) > 0 {
			messagesForDeployment := getMessagesForDeployment(messages, expectedSequence[0].InstanceID)
			testSuite.Assert().Len(messagesForDeployment, len(expectedSequence))
			assertDeploymentMessagesEqual(messagesForDeployment, expectedSequence, testSuite)
		}
	}
}

func getMessagesForDeployment(
	messages []DeploymentUpdateMessage,
	instanceID string,
) []DeploymentUpdateMessage {
	deploymentMessages := []DeploymentUpdateMessage{}
	for _, message := range messages {
		if message.InstanceID == instanceID {
			deploymentMessages = append(deploymentMessages, message)
		}
	}
	return deploymentMessages
}

func assertDeploymentMessagesEqual(
	messages []DeploymentUpdateMessage,
	expectedMessages []DeploymentUpdateMessage,
	testSuite *suite.Suite,
) {
	for i, message := range messages {
		expected := expectedMessages[i]
		testSuite.Assert().Equal(expected.InstanceID, message.InstanceID)
		testSuite.Assert().Equal(expected.Status, message.Status)
		assertTimestampPresent(expected.UpdateTimestamp, message.UpdateTimestamp, testSuite)
	}
}

// Assert that the finished message is as expected.
//
// This will compare the following fields:
// - InstanceID
// - Status
// - FailureReasons
// - FinishTimestamp (Only checks for presence of timestamp)
// - UpdateTimestamp (Only checks for presence of timestamp)
// - Durations (Only checks for presence of duration fields)
//
// For all numeric fields where the value will not be compared,
// the value should be set to -1 in the expected messages.
func assertFinishedMessage(
	message DeploymentFinishedMessage,
	expected DeploymentFinishedMessage,
	testSuite *suite.Suite,
) {
	testSuite.Assert().Equal(expected.InstanceID, message.InstanceID)
	testSuite.Assert().Equal(expected.Status, message.Status)
	assertFailureReasonsEqual(expected.FailureReasons, message.FailureReasons, testSuite)
	assertTimestampPresent(expected.FinishTimestamp, message.FinishTimestamp, testSuite)
	assertTimestampPresent(expected.UpdateTimestamp, message.UpdateTimestamp, testSuite)
	assertFinishedMessageDurations(expected.Durations, message.Durations, testSuite)
}

func assertFinishedMessageDurations(
	expectedDurations *state.InstanceCompletionDuration,
	actualDurations *state.InstanceCompletionDuration,
	testSuite *suite.Suite,
) {
	if expectedDurations != nil {
		testSuite.Assert().NotNil(actualDurations)
		if expectedDurations.TotalDuration != nil {
			testSuite.Assert().NotNil(actualDurations.TotalDuration)
		}

		if expectedDurations.PrepareDuration != nil {
			testSuite.Assert().NotNil(actualDurations.PrepareDuration)
		}
	}
}

func assertTimestampPresent(
	expectedTimestamp int64,
	actualTimestamp int64,
	testSuite *suite.Suite,
) {
	if expectedTimestamp != 0 {
		testSuite.Assert().NotEqual(int64(0), actualTimestamp)
	}
}

func assertAttemptDurationsPresent(
	expectedDurations []float64,
	actualDurations []float64,
	testSuite *suite.Suite,
) {
	testSuite.Assert().Len(actualDurations, len(expectedDurations))
}

func assertFailureReasonsEqual(
	expectedFailureReasons []string,
	actualFailureReasons []string,
	testSuite *suite.Suite,
) {
	if expectedFailureReasons != nil {
		testSuite.Assert().Equal(expectedFailureReasons, actualFailureReasons)
	} else {
		testSuite.Assert().Empty(actualFailureReasons)
	}
}

func createBlueprintDeployFixture(
	deployType string,
	fixtureNo int,
	loader Loader,
) (blueprintDeployFixture, error) {
	blueprintContainer, err := loader.Load(
		context.Background(),
		fmt.Sprintf("__testdata/container/%s/blueprint%d.yml", deployType, fixtureNo),
		baseBlueprintParams(),
	)
	if err != nil {
		return blueprintDeployFixture{}, err
	}

	expectedMessagesFilePath := fmt.Sprintf(
		"__testdata/container/%s/expected-messages/blueprint%d.json",
		deployType,
		fixtureNo,
	)
	return createBlueprintDeployFixtureFromFile(blueprintContainer, expectedMessagesFilePath)
}

func createBlueprintDeployFixtureFromFile(
	container BlueprintContainer,
	expectedMessagesFilePath string,
) (blueprintDeployFixture, error) {
	expectedMessagesBytes, err := os.ReadFile(expectedMessagesFilePath)
	if err != nil {
		return blueprintDeployFixture{}, err
	}

	expectedMessages := &expectedMessages{}
	err = json.Unmarshal(expectedMessagesBytes, expectedMessages)
	if err != nil {
		return blueprintDeployFixture{}, err
	}

	return blueprintDeployFixture{
		blueprintContainer: container,
		expected:           expectedMessages,
	}, nil
}

type blueprintDeployFixture struct {
	blueprintContainer BlueprintContainer
	expected           *expectedMessages
}

type expectedMessages struct {
	ResourceDeployUpdateMessages [][]ResourceDeployUpdateMessage `json:"resourceDeployUpdateMessages"`
	ChildDeployUpdateMessages    [][]ChildDeployUpdateMessage    `json:"childDeployUpdateMessages"`
	LinkDeployUpdateMessages     [][]LinkDeployUpdateMessage     `json:"linkDeployUpdateMessages"`
	DeploymentUpdateMessages     [][]DeploymentUpdateMessage     `json:"deploymentUpdateMessages"`
	FinishedMessage              *DeploymentFinishedMessage      `json:"finishedMessage,omitempty"`
}

type actualMessages struct {
	resourceDeployUpdateMessages []ResourceDeployUpdateMessage
	childDeployUpdateMessages    []ChildDeployUpdateMessage
	linkDeployUpdateMessages     []LinkDeployUpdateMessage
	deploymentUpdateMessages     []DeploymentUpdateMessage
	finishedMessage              *DeploymentFinishedMessage
}
