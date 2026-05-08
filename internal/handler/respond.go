package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, message string) {
	log.Printf("HTTP %d: %s", status, message)
	JSON(w, status, map[string]string{"error": message})
}

// ServerError logs the real error server-side and returns a generic 500 to the client.
func ServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("ERROR %s %s: %v", r.Method, r.URL.Path, err)
	JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
