package site_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestAccountEmailRoutesSendAndChangeExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := redisstore.NewMemoryStore()
	h := site.NewWithRedis(testutil.TestConfig(), db, cache, nil, siteTestMailSender{})
	user := testutil.CreateUser(t, db, "route-email-old@test.com", "Password123", "RouteEmail", false)
	if err := db.Settings.Set(t.Context(), "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(t.Context(), "email_verify_ttl", "150"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v2/users/me/email/verification-code", strings.NewReader(`{"email":"route-email-new@test.com"}`))
	req = withUserActor(req, user.ID)
	rec := httptest.NewRecorder()
	h.SendEmailChangeCode(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ttl\":150}\n" {
		t.Fatalf("send email code response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	code, err := cache.GetVerificationCode(t.Context(), "route-email-new@test.com", "email_change")
	if err != nil || len(code) != 8 {
		t.Fatalf("stored email code=%q err=%v", code, err)
	}

	req = httptest.NewRequest(http.MethodPut, "/v2/users/me/email", strings.NewReader(`{"email":"route-email-new@test.com","code":"`+strings.ToLower(code)+`"}`))
	req = withUserActor(req, user.ID)
	rec = httptest.NewRecorder()
	h.ChangeEmail(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("change email response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(t.Context(), user.ID)
	if err != nil || updated == nil || updated.Email != "route-email-new@test.com" {
		t.Fatalf("changed user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetVerificationCode(t.Context(), "route-email-new@test.com", "email_change"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful route must consume code, got %v", err)
	}
}

func TestAccountEmailRoutesRejectMalformedBodiesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := site.NewWithRedis(testutil.TestConfig(), db, redisstore.NewMemoryStore(), nil, siteTestMailSender{})
	user := testutil.CreateUser(t, db, "route-email-malformed@test.com", "Password123", "RouteEmailMalformed", false)

	for _, tc := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "send code", call: h.SendEmailChangeCode},
		{name: "change email", call: h.ChangeEmail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := withUserActor(httptest.NewRequest(http.MethodPost, "/v2/users/me/email", strings.NewReader(`{`)), user.ID)
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
				t.Fatalf("malformed response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
			}
		})
	}
	unchanged, err := db.Users.GetByID(t.Context(), user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email {
		t.Fatalf("malformed requests mutated email: user=%#v err=%v", unchanged, err)
	}
}
