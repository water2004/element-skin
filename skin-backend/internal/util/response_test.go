package util

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONAndErrorResponsesAreExactByFile(t *testing.T) {
	if got := (HTTPError{Object: "request", Operation: "validate", Reason: "invalid"}).Error(); got != "request.validate.invalid" {
		t.Fatalf("HTTPError.Error()=%q, want exact classification", got)
	}

	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]any{"ok": true})
	if rec.Code != http.StatusCreated || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("JSON response mismatch: status=%d content-type=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, YggError{Status: http.StatusForbidden, Code: "ForbiddenOperationException", Message: "Invalid token."})
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"error\":\"ForbiddenOperationException\",\"errorMessage\":\"Invalid token.\"}\n" {
		t.Fatalf("Ygg HTTPError response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, HTTPError{Status: http.StatusTeapot, Object: "request", Operation: "validate", Reason: "invalid"})
	if rec.Code != http.StatusTeapot || rec.Body.String() != "{\"error\":{\"object\":\"request\",\"operation\":\"validate\",\"reason\":\"invalid\"}}\n" {
		t.Fatalf("HTTPError response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	Error(rec, errors.New("database password leaked"))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("generic error should be converged: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestErrorRejectsUnregisteredReasonShape(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, HTTPError{Status: http.StatusBadRequest, Object: "identity", Operation: "link", Reason: "identity_already_linked"})
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"error\":{\"object\":\"server\",\"operation\":\"handle\",\"reason\":\"failed\"}}\n" {
		t.Fatalf("invalid reason must be converged: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestClassifiedErrorPreservesClassificationParamsAndCause(t *testing.T) {
	cause := errors.New("upstream identity unavailable")
	err := ClassifiedError{
		Object:    "identity",
		Operation: "link",
		Reason:    "conflict",
		Params:    map[string]any{"provider": "microsoft"},
		Cause:     cause,
	}
	if got := err.Error(); got != "identity.link.conflict" {
		t.Fatalf("ClassifiedError.Error()=%q, want identity.link.conflict", got)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("ClassifiedError should unwrap its cause: %v", err)
	}

	object, operation, reason, params := ErrorClassification(err)
	if object != "identity" || operation != "link" || reason != "conflict" ||
		len(params) != 1 || params["provider"] != "microsoft" {
		t.Fatalf("classification=(%q,%q,%q,%#v), want exact identity link conflict tuple", object, operation, reason, params)
	}
}

func TestErrorClassificationConvergesMalformedAndUnclassifiedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "unclassified", err: errors.New("database connection failed")},
		{name: "invalid object", err: ClassifiedError{Object: "Identity", Operation: "link", Reason: "conflict"}},
		{name: "invalid operation", err: ClassifiedError{Object: "identity", Operation: "link.profile", Reason: "conflict"}},
		{name: "unregistered reason", err: ClassifiedError{Object: "identity", Operation: "link", Reason: "owned_by_other_user"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			object, operation, reason, params := ErrorClassification(tt.err)
			if object != InternalErrorObject || operation != InternalErrorOperation ||
				reason != InternalErrorReason || params != nil {
				t.Fatalf("classification=(%q,%q,%q,%#v), want exact internal fallback", object, operation, reason, params)
			}
		})
	}
}
