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

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func codeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "UNPROCESSABLE_ENTITY"
	default:
		return "ERROR"
	}
}

func Error(w http.ResponseWriter, status int, message string) {
	log.Printf("HTTP %d: %s", status, message)
	JSON(w, status, apiError{Code: codeFromStatus(status), Message: message, Details: map[string]string{}})
}

func ValidationError(w http.ResponseWriter, message string, details map[string]string) {
	log.Printf("HTTP 422: %s", message)
	JSON(w, http.StatusUnprocessableEntity, apiError{Code: "VALIDATION_ERROR", Message: message, Details: details})
}

// ServerError logs the real error server-side and returns a generic 500 to the client.
func ServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("ERROR %s %s: %v", r.Method, r.URL.Path, err)
	JSON(w, http.StatusInternalServerError, apiError{Code: "INTERNAL_ERROR", Message: "internal server error", Details: map[string]string{}})
}
