package fallback_test

import (
	"context"
	"net/http"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/permission"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestFallbackStoresDependencies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	fb := newFallback(db, nil)
	if fb.DB != db {
		t.Fatal("Fallback should retain DB dependency")
	}
}

func TestFallbackWhitelistServiceExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	fb := newFallback(db, nil)
	actor := fallbackActor("official_whitelist.read.any", "official_whitelist.add.any", "official_whitelist.remove.any")
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: "https://session.example", AccountURL: "https://account.example",
		ServicesURL: "https://services.example", CacheTTL: 60, EnableWhitelist: true, Note: "primary",
	}}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(endpoints) != 1 {
		t.Fatalf("fallback endpoint seed mismatch: endpoints=%#v err=%v", endpoints, err)
	}
	endpointID := endpoints[0]["id"].(int)

	initial, err := fb.ListWhitelistUsers(ctx, actor, endpointID)
	if err != nil {
		t.Fatal(err)
	}
	if len(initial) != 0 {
		t.Fatalf("initial whitelist should be empty slice: %#v", initial)
	}
	if err := fb.AddWhitelistUser(ctx, actor, fallbacksvc.WhitelistInput{Username: "  PlayerOne  ", EndpointID: endpointID}); err != nil {
		t.Fatal(err)
	}
	users, err := fb.ListWhitelistUsers(ctx, actor, endpointID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0]["username"] != "PlayerOne" {
		t.Fatalf("whitelist after add mismatch: %#v", users)
	}
	if ok, err := db.Fallbacks.IsUserInWhitelist(ctx, "PlayerOne", endpointID); err != nil || !ok {
		t.Fatalf("whitelist database state mismatch: ok=%v err=%v", ok, err)
	}
	if err := fb.RemoveWhitelistUser(ctx, actor, "PlayerOne", endpointID); err != nil {
		t.Fatal(err)
	}
	if users, err := fb.ListWhitelistUsers(ctx, actor, endpointID); err != nil || len(users) != 0 {
		t.Fatalf("whitelist after remove mismatch: users=%#v err=%v", users, err)
	}
}

func TestFallbackWhitelistServiceRejectsInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	fb := newFallback(db, nil)
	actor := fallbackActor("official_whitelist.read.any", "official_whitelist.add.any", "official_whitelist.remove.any")

	if _, err := fb.ListWhitelistUsers(ctx, permission.Actor{}, 1); !fallbackHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("ListWhitelistUsers without permission mismatch: %#v", err)
	}
	if _, err := fb.ListWhitelistUsers(ctx, actor, 0); !fallbackHTTPError(err, http.StatusBadRequest, "fallback_endpoint.configure.required") {
		t.Fatalf("ListWhitelistUsers missing endpoint mismatch: %#v", err)
	}
	if err := fb.AddWhitelistUser(ctx, permission.Actor{}, fallbacksvc.WhitelistInput{Username: "Player", EndpointID: 1}); !fallbackHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("AddWhitelistUser without permission mismatch: %#v", err)
	}
	if err := fb.AddWhitelistUser(ctx, actor, fallbacksvc.WhitelistInput{Username: " \t ", EndpointID: 1}); !fallbackHTTPError(err, http.StatusBadRequest, "username.validate.required") {
		t.Fatalf("AddWhitelistUser blank username mismatch: %#v", err)
	}
	if err := fb.AddWhitelistUser(ctx, actor, fallbacksvc.WhitelistInput{Username: "Player", EndpointID: 0}); !fallbackHTTPError(err, http.StatusBadRequest, "fallback_endpoint.configure.required") {
		t.Fatalf("AddWhitelistUser missing endpoint mismatch: %#v", err)
	}
	if err := fb.AddWhitelistUser(ctx, actor, fallbacksvc.WhitelistInput{Username: "Player", EndpointID: 999999}); !fallbackHTTPError(err, http.StatusNotFound, "fallback_endpoint.resolve.not_found") {
		t.Fatalf("AddWhitelistUser missing endpoint row mismatch: %#v", err)
	}
	if err := fb.RemoveWhitelistUser(ctx, permission.Actor{}, "Player", 1); !fallbackHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("RemoveWhitelistUser without permission mismatch: %#v", err)
	}
	if err := fb.RemoveWhitelistUser(ctx, actor, "Player", 0); !fallbackHTTPError(err, http.StatusBadRequest, "fallback_endpoint.configure.required") {
		t.Fatalf("RemoveWhitelistUser missing endpoint mismatch: %#v", err)
	}
}

func fallbackActor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "fallback-test",
		UserID:      "fallback-test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func fallbackHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Error() == detail
}
