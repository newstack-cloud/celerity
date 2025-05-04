package deploymentsv1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

func (s *ControllerTestSuite) Test_destroy_blueprint_instance() {
	// Create the blueprint instance to be destroyed.
	_, err := s.saveTestBlueprintInstance()
	s.Require().NoError(err)
	// Create the test change set to be used to start the destroy
	// process for the blueprint instance.
	err = s.saveTestChangeset()
	s.Require().NoError(err)

	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances/{id}/destroy",
		s.ctrl.DestroyBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceDestroyRequestPayload{
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	path := fmt.Sprintf(
		"/deployments/instances/%s/destroy",
		testInstanceID,
	)
	req := httptest.NewRequest("POST", path, bytes.NewReader(reqBytes))
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
		core.InstanceStatusDestroying,
		instance.Status,
	)
	s.Assert().Equal(
		testTime.Unix(),
		int64(instance.LastStatusUpdateTimestamp),
	)
}

func (s *ControllerTestSuite) Test_destroy_blueprint_instance_handler_returns_404_not_found() {
	_, err := s.saveTestBlueprintInstance()
	s.Require().NoError(err)

	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances/{id}/destroy",
		s.ctrl.DestroyBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceDestroyRequestPayload{
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	path := fmt.Sprintf("/deployments/instances/%s/destroy", nonExistentInstanceID)
	req := httptest.NewRequest("POST", path, bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	responseError := map[string]string{}
	err = json.Unmarshal(respData, &responseError)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusNotFound, result.StatusCode)
	s.Assert().Equal(
		fmt.Sprintf(
			"blueprint instance %q not found",
			nonExistentInstanceID,
		),
		responseError["message"],
	)
}

func (s *ControllerTestSuite) Test_destroy_blueprint_instance_handler_fails_due_to_missing_changeset() {
	// Create the blueprint instance to be destroyed.
	_, err := s.saveTestBlueprintInstance()
	s.Require().NoError(err)

	// We are not saving the test change set for this test,
	// so it should not be found when the request is made.
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/instances/{id}/destroy",
		s.ctrlFailingIDGenerators.DestroyBlueprintInstanceHandler,
	).Methods("POST")

	reqPayload := &BlueprintInstanceDestroyRequestPayload{
		ChangeSetID: testChangesetID,
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	path := fmt.Sprintf(
		"/deployments/instances/%s/destroy",
		testInstanceID,
	)
	req := httptest.NewRequest("POST", path, bytes.NewReader(reqBytes))
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
