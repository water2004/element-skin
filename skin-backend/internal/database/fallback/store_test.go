package fallback_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestStoreEndpointsDomainsAndWhitelist(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := fallback.Store{Pool: db.Pool}
	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{
		{Priority: 2, SessionURL: "https://session-two", AccountURL: "https://account-two", ServicesURL: "https://services-two", CacheTTL: 120, SkinDomains: []string{"skins.example", "cdn.example"}, EnableWhitelist: true, Note: "second"},
		{Priority: 1, SessionURL: "https://session-one", AccountURL: "https://account-one", ServicesURL: "https://services-one", CacheTTL: 60, SkinDomains: []string{"cdn.example", "textures.example"}, EnableProfile: true, Note: "first"},
	}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := store.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 || endpoints[0]["priority"] != 1 || endpoints[0]["session_url"] != "https://session-one" || endpoints[1]["enable_whitelist"] != true {
		t.Fatalf("endpoint list mismatch: %#v", endpoints)
	}
	primary, err := store.PrimaryEndpoint(ctx)
	if err != nil || primary["services_url"] != "https://services-one" {
		t.Fatalf("primary mismatch: primary=%#v err=%v", primary, err)
	}
	domains, err := store.CollectSkinDomains(ctx)
	if err != nil || len(domains) != 3 || domains[0] != "cdn.example" || domains[1] != "textures.example" || domains[2] != "skins.example" {
		t.Fatalf("domains mismatch: domains=%#v err=%v", domains, err)
	}
	endpointID := endpoints[1]["id"].(int)
	if ok, err := store.IsUserInWhitelist(ctx, "Alex", endpointID); err != nil || ok {
		t.Fatalf("user should not be whitelisted: ok=%v err=%v", ok, err)
	}
	if err := store.AddWhitelistUser(ctx, "Alex", endpointID); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.IsUserInWhitelist(ctx, "Alex", endpointID); err != nil || !ok {
		t.Fatalf("user should be whitelisted: ok=%v err=%v", ok, err)
	}
	users, err := store.ListWhitelistUsers(ctx, endpointID)
	if err != nil || len(users) != 1 || users[0]["username"] != "Alex" {
		t.Fatalf("whitelist list mismatch: users=%#v err=%v", users, err)
	}
	if err := store.RemoveWhitelistUser(ctx, "Alex", endpointID); err != nil {
		t.Fatal(err)
	}
}

func TestEndpointNotFoundClassifierMatchesOnlyWhitelistForeignKey(t *testing.T) {
	if !fallback.IsEndpointNotFound(&pgconn.PgError{
		Code:           "23503",
		ConstraintName: "whitelisted_users_endpoint_id_fkey",
	}) {
		t.Fatal("whitelist endpoint foreign-key violation should be classified")
	}
	for _, err := range []error{
		&pgconn.PgError{Code: "23503", ConstraintName: "other_fkey"},
		&pgconn.PgError{Code: "23505", ConstraintName: "whitelisted_users_endpoint_id_fkey"},
	} {
		if fallback.IsEndpointNotFound(err) {
			t.Fatalf("unrelated database error was classified as missing endpoint: %#v", err)
		}
	}
}

func TestSaveEndpointsPreservesWhitelistForStableIDsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := fallback.Store{Pool: db.Pool}
	initial := []fallback.Endpoint{
		{Priority: 1, SessionURL: "https://keep.example/session", AccountURL: "https://keep.example/account", ServicesURL: "https://keep.example/services", CacheTTL: 60, SkinDomains: []string{"old.keep.example"}, EnableWhitelist: true, Note: "keep"},
		{Priority: 2, SessionURL: "https://remove.example/session", AccountURL: "https://remove.example/account", ServicesURL: "https://remove.example/services", CacheTTL: 120, SkinDomains: []string{"remove.example"}, EnableProfile: true, EnableHasJoined: true, EnableWhitelist: true, Note: "remove"},
		{Priority: 3, SessionURL: "https://stable.example/session", AccountURL: "https://stable.example/account", ServicesURL: "https://stable.example/services", CacheTTL: 300, SkinDomains: []string{"stable.example", "cdn.stable.example"}, EnableProfile: true, EnableHasJoined: false, EnableWhitelist: true, Note: "stable"},
	}
	if err := store.SaveEndpoints(ctx, initial); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListEndpoints(ctx)
	if err != nil || len(before) != 3 {
		t.Fatalf("initial endpoints=%#v err=%v; want exactly three", before, err)
	}
	keepID := before[0]["id"].(int)
	removeID := before[1]["id"].(int)
	stableID := before[2]["id"].(int)
	for _, username := range []string{"Steve", "Alex"} {
		if err := store.AddWhitelistUser(ctx, username, keepID); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.AddWhitelistUser(ctx, "RemovedPlayer", removeID); err != nil {
		t.Fatal(err)
	}
	if err := store.AddWhitelistUser(ctx, "StablePlayer", stableID); err != nil {
		t.Fatal(err)
	}
	whitelistBefore, err := store.ListWhitelistUsers(ctx, keepID)
	if err != nil || len(whitelistBefore) != 2 {
		t.Fatalf("initial whitelist=%#v err=%v; want exactly two", whitelistBefore, err)
	}

	stableWhitelistBefore, err := store.ListWhitelistUsers(ctx, stableID)
	if err != nil || len(stableWhitelistBefore) != 1 {
		t.Fatalf("stable whitelist=%#v err=%v; want exactly one", stableWhitelistBefore, err)
	}
	updatedKeep := fallback.Endpoint{ID: keepID, Priority: 2, SessionURL: "https://keep.example/session-v2", AccountURL: "https://keep.example/account-v2", ServicesURL: "https://keep.example/services-v2", CacheTTL: 180, SkinDomains: []string{"new.keep.example", "cdn.keep.example"}, EnableProfile: true, EnableHasJoined: false, EnableWhitelist: true, Note: "keep updated"}
	newEndpoint := fallback.Endpoint{Priority: 1, SessionURL: "https://new.example/session", AccountURL: "https://new.example/account", ServicesURL: "https://new.example/services", CacheTTL: 90, SkinDomains: []string{"new.example"}, EnableProfile: false, EnableHasJoined: true, EnableWhitelist: false, Note: "new"}
	stableEndpoint := initial[2]
	stableEndpoint.ID = stableID
	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{updatedKeep, newEndpoint, stableEndpoint}); err != nil {
		t.Fatal(err)
	}

	after, err := store.ListEndpoints(ctx)
	if err != nil || len(after) != 3 {
		t.Fatalf("updated endpoints=%#v err=%v; want exactly three", after, err)
	}
	newID := after[0]["id"].(int)
	if newID == keepID || newID == removeID || newID == stableID {
		t.Fatalf("new endpoint identity mismatch: %#v", after[0])
	}
	assertEndpointMapExact(t, after[0], newID, newEndpoint)
	assertEndpointMapExact(t, after[1], keepID, updatedKeep)
	assertEndpointMapExact(t, after[2], stableID, stableEndpoint)
	whitelistAfter, err := store.ListWhitelistUsers(ctx, keepID)
	if err != nil || !reflect.DeepEqual(whitelistAfter, whitelistBefore) {
		t.Fatalf("stable endpoint whitelist=%#v err=%v; want exact preservation %#v", whitelistAfter, err, whitelistBefore)
	}
	removedWhitelist, err := store.ListWhitelistUsers(ctx, removeID)
	if err != nil || len(removedWhitelist) != 0 {
		t.Fatalf("removed endpoint whitelist=%#v err=%v; want empty", removedWhitelist, err)
	}
	stableWhitelistAfter, err := store.ListWhitelistUsers(ctx, stableID)
	if err != nil || !reflect.DeepEqual(stableWhitelistAfter, stableWhitelistBefore) {
		t.Fatalf("unchanged endpoint whitelist=%#v err=%v; want exact preservation %#v", stableWhitelistAfter, err, stableWhitelistBefore)
	}
}

func assertEndpointMapExact(t *testing.T, got map[string]any, id int, want fallback.Endpoint) {
	t.Helper()
	if got["id"] != id ||
		got["priority"] != want.Priority ||
		got["session_url"] != want.SessionURL ||
		got["account_url"] != want.AccountURL ||
		got["services_url"] != want.ServicesURL ||
		got["cache_ttl"] != want.CacheTTL ||
		!reflect.DeepEqual(got["skin_domains"], want.SkinDomains) ||
		got["enable_profile"] != want.EnableProfile ||
		got["enable_hasjoined"] != want.EnableHasJoined ||
		got["enable_whitelist"] != want.EnableWhitelist ||
		got["note"] != want.Note {
		t.Fatalf("endpoint mismatch: got=%#v want_id=%d want=%#v", got, id, want)
	}
}

func TestSaveEndpointsRejectsUnknownAndDuplicateIDsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := fallback.Store{Pool: db.Pool}
	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{{Priority: 1, SessionURL: "https://stable.example/session", AccountURL: "https://stable.example/account", ServicesURL: "https://stable.example/services", CacheTTL: 60, Note: "stable"}}); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListEndpoints(ctx)
	if err != nil || len(before) != 1 {
		t.Fatalf("initial endpoints=%#v err=%v", before, err)
	}
	stableID := before[0]["id"].(int)

	err = store.SaveEndpoints(ctx, []fallback.Endpoint{{ID: stableID + 1000, Priority: 1, SessionURL: "https://unknown.example/session", AccountURL: "https://unknown.example/account", ServicesURL: "https://unknown.example/services", CacheTTL: 60}})
	if !errors.Is(err, fallback.ErrEndpointNotFound) {
		t.Fatalf("unknown endpoint error=%#v; want ErrEndpointNotFound", err)
	}
	err = store.SaveEndpoints(ctx, []fallback.Endpoint{
		{ID: stableID, Priority: 1, SessionURL: "https://first.example/session", AccountURL: "https://first.example/account", ServicesURL: "https://first.example/services", CacheTTL: 60},
		{ID: stableID, Priority: 2, SessionURL: "https://second.example/session", AccountURL: "https://second.example/account", ServicesURL: "https://second.example/services", CacheTTL: 60},
	})
	if !errors.Is(err, fallback.ErrDuplicateEndpoint) {
		t.Fatalf("duplicate endpoint error=%#v; want ErrDuplicateEndpoint", err)
	}
	after, err := store.ListEndpoints(ctx)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("rejected saves changed endpoints: after=%#v before=%#v err=%v", after, before, err)
	}
}

func TestSaveEndpointsEmptyListRemovesAllEndpointStateExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := fallback.Store{Pool: db.Pool}
	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{
		{Priority: 1, SessionURL: "https://one.example/session", AccountURL: "https://one.example/account", ServicesURL: "https://one.example/services", SkinDomains: []string{"one.example"}, EnableWhitelist: true},
		{Priority: 2, SessionURL: "https://two.example/session", AccountURL: "https://two.example/account", ServicesURL: "https://two.example/services", SkinDomains: []string{"two.example"}, EnableWhitelist: true},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListEndpoints(ctx)
	if err != nil || len(before) != 2 {
		t.Fatalf("initial endpoints=%#v err=%v; want exactly two", before, err)
	}
	endpointIDs := []int{before[0]["id"].(int), before[1]["id"].(int)}
	for index, endpointID := range endpointIDs {
		if err := store.AddWhitelistUser(ctx, []string{"FirstPlayer", "SecondPlayer"}[index], endpointID); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{}); err != nil {
		t.Fatal(err)
	}
	after, err := store.ListEndpoints(ctx)
	if err != nil || len(after) != 0 {
		t.Fatalf("empty save endpoints=%#v err=%v; want empty", after, err)
	}
	domains, err := store.CollectSkinDomains(ctx)
	if err != nil || len(domains) != 0 {
		t.Fatalf("empty save domains=%#v err=%v; want empty", domains, err)
	}
	for _, endpointID := range endpointIDs {
		users, err := store.ListWhitelistUsers(ctx, endpointID)
		if err != nil || len(users) != 0 {
			t.Fatalf("empty save endpoint %d whitelist=%#v err=%v; want empty", endpointID, users, err)
		}
	}
}
