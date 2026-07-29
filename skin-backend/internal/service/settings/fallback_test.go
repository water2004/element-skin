package settings

import (
	"reflect"
	"testing"

	"element-skin/backend/internal/util"
)

func TestValidateFallbackEndpointsRejectsMissingURLs(t *testing.T) {
	_, err := ValidateFallbackEndpoints([]any{map[string]any{
		"session_url":  "https://session.example",
		"account_url":  "",
		"services_url": "https://services.example",
	}})
	httpErr, ok := err.(util.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T %[1]v", err)
	}
	if httpErr.Status != 400 || httpErr.Detail != "fallback[1] urls are required" {
		t.Fatalf("unexpected error: %#v", httpErr)
	}
}

func TestFallbackNormalizationHelpersExactValues(t *testing.T) {
	if got := intValue("7", 1); got != 7 {
		t.Fatalf("intValue string mismatch: %d", got)
	}
	if got := intValue(float64(8), 1); got != 8 {
		t.Fatalf("intValue float mismatch: %d", got)
	}
	if got := intValue("bad", 9); got != 9 {
		t.Fatalf("intValue fallback mismatch: %d", got)
	}
	if !boolValue("1") || !boolValue(float64(1)) || boolValue("false") || boolValue(0) {
		t.Fatalf("boolValue exact coercion failed")
	}
	got, err := normalizeDomains(3, []any{" skins.example ", "", "cdn.example", "skins.example"})
	if err != nil || !reflect.DeepEqual(got, []string{"skins.example", "cdn.example"}) {
		t.Fatalf("normalizeDomains list mismatch: %#v", got)
	}
	if got := valueOr(nil, "fallback"); got != "fallback" {
		t.Fatalf("valueOr nil mismatch: %#v", got)
	}
	if got := valueOr("value", "fallback"); got != "value" {
		t.Fatalf("valueOr value mismatch: %#v", got)
	}
}

func TestValidateFallbackInputsRejectInvalidShapesExactly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func() error
		detail string
	}{
		{
			name: "endpoints must be list",
			call: func() error {
				_, err := ValidateFallbackEndpoints(map[string]any{})
				return err
			},
			detail: "fallbacks must be a list",
		},
		{
			name: "endpoint must be object",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{"not-an-object"})
				return err
			},
			detail: "invalid fallback entry",
		},
		{
			name: "endpoint id must be positive integer",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{map[string]any{
					"id":           float64(1.5),
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "https://services.example",
				}})
				return err
			},
			detail: "fallback[1] id must be a positive integer",
		},
		{
			name: "negative cache TTL",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{map[string]any{
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "https://services.example",
					"cache_ttl":    -1,
				}})
				return err
			},
			detail: "fallback[1] cache_ttl must be non-negative",
		},
		{
			name: "skin domains must be list",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{map[string]any{
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "https://services.example",
					"skin_domains": "skins.example",
				}})
				return err
			},
			detail: "fallback[1] skin_domains must be a list",
		},
		{
			name: "skin domains must contain strings",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{map[string]any{
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "https://services.example",
					"skin_domains": []any{"skins.example", 7},
				}})
				return err
			},
			detail: "fallback[1] skin_domains must contain only strings",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			httpErr, ok := err.(util.HTTPError)
			if !ok || httpErr.Status != 400 || httpErr.Detail != tc.detail {
				t.Fatalf("error=%#v; want exact HTTP 400 detail %q", err, tc.detail)
			}
		})
	}
}

func TestValidateFallbackEndpointsNormalizesJSONCompatibleValuesExactly(t *testing.T) {
	endpoints, err := ValidateFallbackEndpoints([]any{map[string]any{
		"id":               float64(42),
		"priority":         int64(4),
		"session_url":      " https://session.example ",
		"account_url":      " https://account.example ",
		"services_url":     " https://services.example ",
		"cache_ttl":        int64(90),
		"skin_domains":     []string{" skins.example ", "", "cdn.example", "skins.example"},
		"enable_profile":   true,
		"enable_hasjoined": 0,
		"enable_whitelist": float64(1),
		"note":             " primary ",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		42,
		4,
		"https://session.example",
		"https://account.example",
		"https://services.example",
		90,
		[]string{"skins.example", "cdn.example"},
		true,
		false,
		true,
		"primary",
	}
	got := endpoints[0]
	actual := []any{
		got.ID,
		got.Priority,
		got.SessionURL,
		got.AccountURL,
		got.ServicesURL,
		got.CacheTTL,
		got.SkinDomains,
		got.EnableProfile,
		got.EnableHasJoined,
		got.EnableWhitelist,
		got.Note,
	}
	if len(endpoints) != 1 || !reflect.DeepEqual(actual, want) {
		t.Fatalf("normalized endpoint=%#v; want exact values %#v", got, want)
	}
}

func TestValidateFallbackEndpointsRejectsDuplicateIDsExactly(t *testing.T) {
	entry := func() map[string]any {
		return map[string]any{
			"id":           float64(42),
			"session_url":  "https://session.example",
			"account_url":  "https://account.example",
			"services_url": "https://services.example",
		}
	}
	_, err := ValidateFallbackEndpoints([]any{entry(), entry()})
	httpErr, ok := err.(util.HTTPError)
	if !ok || httpErr.Status != 400 || httpErr.Detail != "fallback[2] id is duplicated" {
		t.Fatalf("duplicate endpoint ID error=%#v; want exact HTTP 400", err)
	}
}
