package validationv1

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/mux"
)

func (s *ControllerTestSuite) Test_cleanup_blueprint_validation_handler() {
	router := mux.NewRouter()
	router.HandleFunc(
		"/validations/cleanup",
		s.ctrl.CleanupBlueprintValidationsHandler,
	).Methods("POST")

	req := httptest.NewRequest("POST", "/validations/cleanup", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	result := w.Result()
	defer result.Body.Close()
	respData, err := io.ReadAll(result.Body)
	s.Require().NoError(err)

	response := map[string]string{}
	err = json.Unmarshal(respData, &response)
	s.Require().NoError(err)

	s.Assert().Equal(http.StatusAccepted, result.StatusCode)
	s.Assert().Equal(
		"Cleanup started",
		response["message"],
	)
}
