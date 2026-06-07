package util

import (
	"encoding/json"
	"net/http"
)

const InternalServerErrorDetail = "Internal server error"

type HTTPError struct {
	Status   int
	Detail   string
	YggError string
}

func (e HTTPError) Error() string { return e.Detail }

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, err error) {
	if he, ok := err.(HTTPError); ok {
		if he.YggError != "" {
			JSON(w, he.Status, map[string]any{"error": he.YggError, "errorMessage": he.Detail})
			return
		}
		JSON(w, he.Status, map[string]any{"detail": he.Detail})
		return
	}
	JSON(w, http.StatusInternalServerError, map[string]any{"detail": InternalServerErrorDetail})
}
