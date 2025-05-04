package deploymentsv1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/enginev1/inputvalidation"
	"github.com/two-hundred/celerity/apps/deploy-engine/internal/resolve"
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
