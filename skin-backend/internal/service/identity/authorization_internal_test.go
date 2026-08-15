package identity

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func TestAuthorizationStateDecodersHandlePersistedRepresentationsExactly(t *testing.T) {
	state := map[string]any{
		"native_integer":  int64(41),
		"string_integer":  "42",
		"invalid_integer": true,
		"native_strings":  []string{"openid", "profile"},
		"mixed_strings":   []any{"openid", 3, "email"},
	}

	if got := stateInt64(state, "native_integer"); got != 41 {
		t.Fatalf("native integer=%d want=41", got)
	}
	if got := stateInt64(state, "string_integer"); got != 42 {
		t.Fatalf("string integer=%d want=42", got)
	}
	if got := stateInt64(state, "invalid_integer"); got != 0 {
		t.Fatalf("invalid integer=%d want=0", got)
	}
	if got := stateStrings(state, "native_strings"); !reflect.DeepEqual(got, []string{"openid", "profile"}) {
		t.Fatalf("native strings=%v", got)
	}
	if got := stateStrings(state, "mixed_strings"); !reflect.DeepEqual(got, []string{"openid", "email"}) {
		t.Fatalf("mixed strings=%v", got)
	}
	if got := stateStrings(state, "missing_strings"); !reflect.DeepEqual(got, []string{}) {
		t.Fatalf("missing strings=%v want empty non-nil slice", got)
	}
}

func TestAuthorizationHelpersPreserveFallbackAndErrorContractsExactly(t *testing.T) {
	service := Service{Config: config.Config{SiteURL: "https://site.example/"}}
	if got := service.RedirectURI(); got != "https://site.example/v2/auth/oidc/callback" {
		t.Fatalf("site fallback redirect URI=%q", got)
	}

	if code, ok := AuthorizationLinkErrorCode(errors.New("plain error")); ok || code != "" {
		t.Fatalf("plain link error code=%q ok=%v", code, ok)
	}
	if code, ok := AuthorizationLinkErrorCode(util.HTTPError{Status: 400, Detail: "unrelated"}); ok || code != "" {
		t.Fatalf("unrelated HTTP link error code=%q ok=%v", code, ok)
	}

	if _, err := (Service{Config: config.Config{IdentityEncryptionKey: "invalid"}}).credential("identity", OIDCTokens{RefreshToken: "refresh"}, 1000); err == nil || err.Error() != "decode identity encryption key: illegal base64 data at input byte 4" {
		t.Fatalf("invalid credential encryption error=%v", err)
	}
}

func TestCacheAccessTokenAssignsBoundedFallbackExpiryExactly(t *testing.T) {
	cache := redisstore.NewMemoryStore()
	service := Service{Redis: cache}
	tokens := OIDCTokens{AccessToken: "access-token", TokenType: "Bearer"}
	before := time.Now().Add(5 * time.Minute).UnixMilli()
	if err := service.cacheAccessToken(context.Background(), "identity-fallback", tokens); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(5 * time.Minute).UnixMilli()
	stored, err := cache.GetExternalAccessToken(context.Background(), "identity-fallback")
	if err != nil {
		t.Fatal(err)
	}
	if stored.IdentityID != "identity-fallback" || stored.AccessToken != "access-token" || stored.TokenType != "Bearer" ||
		stored.ExpiresAt < before || stored.ExpiresAt > after {
		t.Fatalf("fallback access token=%#v expiry bounds=[%d,%d]", stored, before, after)
	}
}
