package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONAndErrorResponsesAreExactByFile(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]any{"ok": true})
	if rec.Code != http.StatusCreated || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("JSON response mismatch: status=%d content-type=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, HTTPError{Status: http.StatusForbidden, Detail: "Invalid token.", YggError: "ForbiddenOperationException"})
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":\"ForbiddenOperationException\",\"errorMessage\":\"Invalid token.\"}\n" {
		t.Fatalf("Ygg HTTPError response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, errors.New("database password leaked"))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("generic error should be converged: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
