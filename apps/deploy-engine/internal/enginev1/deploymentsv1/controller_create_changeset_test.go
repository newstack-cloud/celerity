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
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

func (s *ControllerTestSuite) Test_create_changeset_handler() {
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/changes",
		s.ctrl.CreateChangesetHandler,
	).Methods("POST")

	reqPayload := &CreateChangesetRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			FileSourceScheme: "file",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/changes", bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	changeset := &manage.Changeset{}
	err = json.Unmarshal(respData, changeset)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusAccepted, result.StatusCode)
	_, err = uuid.Parse(changeset.ID)
	s.Assert().NoError(err, "ID should be a valid UUID (as per the configured generator)")
	s.Assert().Equal(
		manage.ChangesetStatusStarting,
		changeset.Status,
	)
	s.Assert().Equal(
		"file:///test/dir/test.blueprint.yaml",
		changeset.BlueprintLocation,
	)
	s.Assert().Equal(
		testTime.Unix(),
		changeset.Created,
	)
}

func (s *ControllerTestSuite) Test_create_changeset_handler_fails_for_invalid_input() {
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/changes",
		s.ctrl.CreateChangesetHandler,
	).Methods("POST")

	reqPayload := &CreateChangesetRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			// "files" is not a valid scheme.
			FileSourceScheme: "files",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/changes", bytes.NewReader(reqBytes))
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

func (s *ControllerTestSuite) Test_create_changeset_handler_fails_due_to_id_gen_error() {
	router := mux.NewRouter()
	router.HandleFunc(
		"/deployments/changes",
		s.ctrlFailingIDGenerators.CreateChangesetHandler,
	).Methods("POST")

	reqPayload := &CreateChangesetRequestPayload{
		BlueprintDocumentInfo: resolve.BlueprintDocumentInfo{
			FileSourceScheme: "file",
			Directory:        "/test/dir",
			BlueprintFile:    "test.blueprint.yaml",
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	s.Require().NoError(err)

	req := httptest.NewRequest("POST", "/deployments/changes", bytes.NewReader(reqBytes))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	responseError := map[string]string{}
	err = json.Unmarshal(respData, &responseError)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusInternalServerError, result.StatusCode)
	s.Assert().Equal(
		"an unexpected error occurred",
		responseError["message"],
	)
}
