package enginev1

import (
	"fmt"
	"net/http"
	"time"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	respString := fmt.Sprintf("{\"timestamp\":%d}", time.Now().Unix())
	w.Write([]byte(respString))
}
