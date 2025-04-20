package httputils

import (
	"encoding/json"
	"net/http"
)

// HTTPError writes http error responses with a message represented in a JSON object.
func HTTPError(w http.ResponseWriter, statusCode int, message string) {
	HTTPErrorWithFields(w, statusCode, message, map[string]any{})
}

// HTTPErrorWithFields writes http error responses with a message represented in a JSON object
// along with extra fields.
func HTTPErrorWithFields(w http.ResponseWriter, statusCode int, message string, fields map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fields["message"] = message
	errorResponse, _ := json.Marshal(fields)
	w.Write(errorResponse)
}
