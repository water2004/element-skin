package identity

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"element-skin/backend/internal/util"
)

func TestAuthorizationErrorQueryPreservesStructuredIdentityErrorsExactly(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		want      url.Values
		redirects bool
	}{
		{
			name:      "same account already linked",
			err:       util.HTTPError{Status: http.StatusConflict, Object: "identity", Operation: "link", Reason: "already_exists"},
			want:      url.Values{"error_object": {"identity"}, "error_operation": {"link"}, "error_reason": {"already_exists"}},
			redirects: true,
		},
		{
			name:      "other account conflict",
			err:       util.HTTPError{Status: http.StatusConflict, Object: "identity", Operation: "link", Reason: "conflict"},
			want:      url.Values{"error_object": {"identity"}, "error_operation": {"link"}, "error_reason": {"conflict"}},
			redirects: true,
		},
		{
			name:      "authorization mismatch",
			err:       util.HTTPError{Status: http.StatusConflict, Object: "identity", Operation: "authorize", Reason: "mismatch"},
			want:      url.Values{"error_object": {"identity"}, "error_operation": {"authorize"}, "error_reason": {"mismatch"}},
			redirects: true,
		},
		{
			name:      "authorization incomplete",
			err:       util.HTTPError{Status: http.StatusBadRequest, Object: "identity", Operation: "authorize", Reason: "incomplete"},
			want:      url.Values{"error_object": {"identity"}, "error_operation": {"authorize"}, "error_reason": {"incomplete"}},
			redirects: true,
		},
		{name: "unrelated API error", err: util.HTTPError{Status: http.StatusBadRequest, Object: "request", Operation: "decode", Reason: "invalid"}},
		{name: "unclassified error", err: errors.New("database unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := authorizationErrorQuery(tt.err)
			if ok != tt.redirects || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("authorizationErrorQuery()=(%#v,%v), want (%#v,%v)", got, ok, tt.want, tt.redirects)
			}
		})
	}
}
