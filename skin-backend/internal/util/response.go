package util

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
)

const (
	InternalErrorObject    = "server"
	InternalErrorOperation = "handle"
	InternalErrorReason    = "failed"
)

var errorPartPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

var validErrorReasons = map[string]struct{}{
	"required":       {},
	"invalid":        {},
	"not_found":      {},
	"already_exists": {},
	"conflict":       {},
	"denied":         {},
	"expired":        {},
	"disabled":       {},
	"unavailable":    {},
	"failed":         {},
	"mismatch":       {},
	"unsupported":    {},
	"incomplete":     {},
	"too_large":      {},
	"too_long":       {},
	"out_of_range":   {},
	"exhausted":      {},
	"exceeded":       {},
}

type HTTPError struct {
	Status    int
	Object    string
	Operation string
	Reason    string
	Params    map[string]any
}

func (e HTTPError) Error() string { return e.Object + "." + e.Operation + "." + e.Reason }

type ClassifiedError struct {
	Object    string
	Operation string
	Reason    string
	Params    map[string]any
	Cause     error
}

func (e ClassifiedError) Error() string {
	return e.Object + "." + e.Operation + "." + e.Reason
}
func (e ClassifiedError) Unwrap() error { return e.Cause }

func ErrorClassification(err error) (string, string, string, map[string]any) {
	var classified ClassifiedError
	if errors.As(err, &classified) && ValidErrorPart(classified.Object) &&
		ValidErrorPart(classified.Operation) && ValidErrorReason(classified.Reason) {
		return classified.Object, classified.Operation, classified.Reason, classified.Params
	}
	return InternalErrorObject, InternalErrorOperation, InternalErrorReason, nil
}

func ValidErrorPart(value string) bool { return errorPartPattern.MatchString(value) }

func ValidErrorReason(reason string) bool {
	_, ok := validErrorReasons[reason]
	return ok
}

type YggError struct {
	Status  int
	Code    string
	Message string
}

func (e YggError) Error() string { return e.Code }

type ErrorBody struct {
	Object    string         `json:"object"`
	Operation string         `json:"operation"`
	Reason    string         `json:"reason"`
	Params    map[string]any `json:"params,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func Error(w http.ResponseWriter, err error) {
	var yggErr YggError
	if errors.As(err, &yggErr) {
		JSON(w, yggErr.Status, map[string]any{"error": yggErr.Code, "errorMessage": yggErr.Message})
		return
	}
	var httpErr HTTPError
	if errors.As(err, &httpErr) && ValidErrorPart(httpErr.Object) &&
		ValidErrorPart(httpErr.Operation) && ValidErrorReason(httpErr.Reason) {
		JSON(w, httpErr.Status, ErrorResponse{Error: ErrorBody{
			Object: httpErr.Object, Operation: httpErr.Operation, Reason: httpErr.Reason, Params: httpErr.Params,
		}})
		return
	}
	JSON(w, http.StatusInternalServerError, ErrorResponse{Error: ErrorBody{
		Object: InternalErrorObject, Operation: InternalErrorOperation, Reason: InternalErrorReason,
	}})
}
