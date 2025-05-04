package deploymentsv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/r3labs/sse/v2"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/enginev1/inputvalidation"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/resolve"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/testutils"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/container"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func (s *ControllerTestSuite) Test_create_blueprint_instance() {
	// Create the test change set to be used to start the deployment
	// process for the new blueprint instance.
	err := s.saveTestChangeset()
	s.Require().NoError(err)

	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances",
		s.ctrl.CreateBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			FileSourceScheme: "file",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/instances", bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	instance := &state.InstanceState{}
	err = json.Unmarshal(respData, instance)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusAccepted, result.StatusCode)
	_, err = uuid.Parse(instance.InstanceID)
	s.Assert().NoError(err, "ID should be a valid UUID (as per the configured generator)")
	s.Assert().Equal(
		core.InstanceStatusPreparing,
		instance.Status,
	)
	s.Assert().Equal(
		testTime.Unix(),
		int64(instance.LastStatusUpdateTimestamp),
	)

	expectedEvents := deployEventSequence(instance.InstanceID)
	actualEvents, err := s.streamDeployEvents(instance.InstanceID, len(expectedEvents))
	s.Require().NoError(err)

	s.Assert().Len(actualEvents, len(expectedEvents))
	s.Assert().Equal(
		expectedEvents,
		actualEvents,
	)
}

func (s *ControllerTestSuite) Test_create_blueprint_instance_handler_fails_for_invalid_input() {
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances",
		s.ctrl.CreateBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			// "files" is not a valid scheme.
			FileSourceScheme: "files",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/instances", bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	validationError := &inputvalidation.FormattedValidationError{}
	err = json.Unmarshal(respData, validationError)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusUnprocessableEntity, result.StatusCode)
	s.Assert().Equal(
		"request body input validation failed",
		validationError.Message,
	)
	s.Assert().Len(validationError.Errors, 1)
	s.Assert().Equal(
		".fileSourceScheme",
		validationError.Errors[0].Location,
	)
	s.Assert().Equal(
		"the value must be one of the following: file s3 gcs azureblob https",
		validationError.Errors[0].Message,
	)
	s.Assert().Equal(
		"oneof",
		validationError.Errors[0].Type,
	)
}

func (s *ControllerTestSuite) Test_create_blueprint_instance_handler_fails_due_to_missing_changeset() {
	// We are not saving the test change set for this test,
	// so it should not be found when the request is made.
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances",
		s.ctrlFailingIDGenerators.CreateBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			FileSourceScheme: "file",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/instances", bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	responseError := map[string]string{}
	err = json.Unmarshal(respData, &responseError)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusBadRequest, result.StatusCode)
	s.Assert().Equal(
		"requested change set is missing",
		responseError["message"],
	)
}

func (s *ControllerTestSuite) streamDeployEvents(
	instanceID string,
	expectedCount int,
) ([]container.DeployEvent, error) {
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances/{id}/stream",
		s.ctrl.StreamDeploymentEventsHandler,
	).Methods("GET")

	// We need to create a server to be able to stream events asynchronously,
	// the httptest recording test tools do not support response streaming
	// as the Result() method is to be used after response writing is done.
	streamServer := httptest.NewServer(router)
	defer streamServer.Close()

	url := fmt.Sprintf(
		"%s/deployments/instances/%s/stream",
		streamServer.URL,
		instanceID,
	)

	client := sse.NewClient(url)

	eventChan := make(chan *sse.Event)
	client.SubscribeChan("messages", eventChan)
	defer client.Unsubscribe(eventChan)

	collected := []*manage.Event{}
	for len(collected) < expectedCount {
		select {
		case event := <-eventChan:
			manageEvent := testutils.SSEToManageEvent(event)
			collected = append(collected, manageEvent)
			s.Require().NotNil(event)
		case <-time.After(5 * time.Second):
			s.Fail("Timed out waiting for events")
		}
	}

	return extractDeployEvents(collected)
}

func extractDeployEvents(
	collected []*manage.Event,
) ([]container.DeployEvent, error) {
	extractedEvents := []container.DeployEvent{}

	for _, event := range collected {
		if event.Type == eventTypeResourceUpdate {
			resourceUpdate := &container.ResourceDeployUpdateMessage{}
			err := parseEventJSON(event, resourceUpdate)
			if err != nil {
				return nil, err
			}
			extractedEvents = append(extractedEvents, container.DeployEvent{
				ResourceUpdateEvent: resourceUpdate,
			})
		}

		if event.Type == eventTypeChildUpdate {
			childUpdate := &container.ChildDeployUpdateMessage{}
			err := parseEventJSON(event, childUpdate)
			if err != nil {
				return nil, err
			}
			extractedEvents = append(extractedEvents, container.DeployEvent{
				ChildUpdateEvent: childUpdate,
			})
		}

		if event.Type == eventTypeLinkUpdate {
			linkUpdate := &container.LinkDeployUpdateMessage{}
			err := parseEventJSON(event, linkUpdate)
			if err != nil {
				return nil, err
			}
			extractedEvents = append(extractedEvents, container.DeployEvent{
				LinkUpdateEvent: linkUpdate,
			})
		}

		if event.Type == eventTypeInstanceUpdate {
			deploymentUpdate := &container.DeploymentUpdateMessage{}
			err := parseEventJSON(event, deploymentUpdate)
			if err != nil {
				return nil, err
			}
			extractedEvents = append(extractedEvents, container.DeployEvent{
				DeploymentUpdateEvent: deploymentUpdate,
			})
		}

		if event.Type == eventTypeDeployFinished {
			finishEvent := &container.DeploymentFinishedMessage{}
			err := parseEventJSON(event, finishEvent)
			if err != nil {
				return nil, err
			}
			extractedEvents = append(extractedEvents, container.DeployEvent{
				FinishEvent: finishEvent,
			})
		}
	}

	return extractedEvents, nil
}

func parseEventJSON(
	event *manage.Event,
	target any,
) error {
	return json.Unmarshal(
		[]byte(event.Data),
		target,
	)
}
