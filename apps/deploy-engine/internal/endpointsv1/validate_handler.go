package endpointsv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/two-hundred/celerity/apps/deploy-engine/core"
)

type validateHandler struct {
	deployEngine core.DeployEngine
}

// StreamHandler is the handler for the /validate/stream endpoint
func (h *validateHandler) StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the ResponseWriter supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	resultChan := make(chan *core.ValidateResult)
	errChan := make(chan error)

	defer close(resultChan)
	defer close(errChan)

	params := &core.ValidateParams{}
	err := json.NewDecoder(r.Body).Decode(params)
	if err != nil {
		http.Error(w, "Invalid request body!", http.StatusBadRequest)
		return
	}

	go h.deployEngine.ValidateStream(r.Context(), params, resultChan, errChan)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

L:
	for {
		select {
		case <-r.Context().Done():
			break L

		// Listen for incoming messages from messageChan
		case msg := <-resultChan:
			fmt.Printf("Writing message start %+v", msg)
			if msg == nil {
				break L
			}

			// Write to the ResponseWriter
			// Server Sent Events compatible
			fmt.Printf("Writing message %+v", msg)
			eventBytes, _ := json.Marshal(msg)
			fmt.Fprint(w, "event: result\n")
			data := fmt.Sprintf("data: %s\n\n", string(eventBytes))
			fmt.Fprint(w, data)
			// Flush the data immediatly instead of buffering it for later.
			flusher.Flush()

		case err := <-errChan:
			errBytes, _ := json.Marshal(struct {
				Error string `json:"error"`
			}{Error: err.Error()})
			fmt.Fprintf(w, "event: error\n")
			fmt.Fprintf(w, "data: %s\n\n", string(errBytes))
			flusher.Flush()
			break L
		}
	}
}
