package oauth_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServicePublicClientSecretAndInputValidationPathsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-inputs@test.com", "Password123", "OAuthInputs", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	cases := []struct {
		name   string
		input  oauth.ClientInput
		status int
		detail string
	}{
		{name: "empty name", input: oauth.ClientInput{Name: "", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "name.validate.invalid"},
		{name: "bad redirect", input: oauth.ClientInput{Name: "Bad redirect", RedirectURI: "ftp://app.example/callback", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "redirect_uri.validate.invalid"},
		{name: "redirect with fragment", input: oauth.ClientInput{Name: "Fragment redirect", RedirectURI: "https://app.example/callback#token", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "redirect_uri.validate.invalid"},
		{name: "redirect with credentials", input: oauth.ClientInput{Name: "Credential redirect", RedirectURI: "https://user@app.example/callback", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "redirect_uri.validate.invalid"},
		{name: "bad website", input: oauth.ClientInput{Name: "Bad website", RedirectURI: "https://app.example/callback", WebsiteURL: "://bad", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "website_url.validate.invalid"},
		{name: "website with fragment", input: oauth.ClientInput{Name: "Fragment website", RedirectURI: "https://app.example/callback", WebsiteURL: "https://app.example/#section", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "website_url.validate.invalid"},
		{name: "bad type", input: oauth.ClientInput{Name: "Bad type", RedirectURI: "https://app.example/callback", ClientType: "native", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "oauth_client_type.validate.invalid"},
		{name: "bad scope", input: oauth.ClientInput{Name: "Bad scope", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"permission.catalog.system"}}, status: 400, detail: "oauth_scope.validate.invalid"},
		{name: "missing actor scope", input: oauth.ClientInput{Name: "Missing actor scope", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"account.ban.any"}}, status: 403, detail: "permission.check.denied"},
		{name: "public server scope", input: oauth.ClientInput{Name: "Public server scope", RedirectURI: "https://server-public.example/callback", ClientType: oauth.ClientTypePublic, PermissionCodes: []string{"minecraft_session.hasjoined.server"}}, status: 400, detail: "oauth_scope.authorize.denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateClient(ctx, actor, tc.input)
			assertHTTPError(t, err, tc.status, tc.detail)
		})
	}
	serverClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Server scope app",
		RedirectURI:     "https://server-scope.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	serverPermissions := serverClient["permissions"].([]string)
	if len(serverPermissions) != 2 ||
		serverPermissions[0] != "account.read.self" ||
		serverPermissions[1] != "minecraft_session.hasjoined.server" {
		t.Fatalf("server scope application permissions mismatch: %#v", serverPermissions)
	}
	publicClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Public no secret",
		RedirectURI:     "https://public.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := publicClient["client_id"].(string)
	if publicClient["client_secret"] != nil {
		t.Fatalf("public client should not expose a secret: %#v", publicClient)
	}
	if _, err := svc.RotateClientSecret(ctx, actor, clientID); !isHTTPError(err, 400, "client_secret.configure.unsupported") {
		t.Fatalf("rotate public secret error mismatch: %#v", err)
	}
}

type noticeExpectation struct {
	Summary  string
	Content  string
	Level    string
	Audience string
	LinkURL  string
	Target   string
	MinEnds  int64
}

func assertNoticeRow(t *testing.T, db *database.DB, title string, want noticeExpectation) {
	t.Helper()
	var id, summary, content, level, audience, linkURL string
	var endsAt *int64
	err := db.Pool.QueryRow(context.Background(), `
		SELECT id,summary,content_markdown,level,audience,link_url,ends_at
		FROM notices
		WHERE title=$1
	`, title).Scan(&id, &summary, &content, &level, &audience, &linkURL, &endsAt)
	if err != nil {
		t.Fatalf("query notice %q: %v", title, err)
	}
	if id == "" || summary != want.Summary || level != want.Level || audience != want.Audience || linkURL != want.LinkURL {
		t.Fatalf("notice %q fields mismatch: id=%q summary=%q level=%q audience=%q link=%q want=%#v", title, id, summary, level, audience, linkURL, want)
	}
	if want.Content != "" && content != want.Content {
		t.Fatalf("notice %q content mismatch: got=%q want=%q", title, content, want.Content)
	}
	if endsAt == nil || *endsAt < want.MinEnds {
		t.Fatalf("notice %q ends_at=%v want >= %d", title, endsAt, want.MinEnds)
	}
	var targetCount int
	if want.Target == "" {
		err = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notice_targets WHERE notice_id=$1`, id).Scan(&targetCount)
	} else {
		err = db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notice_targets WHERE notice_id=$1 AND user_id=$2`, id, want.Target).Scan(&targetCount)
	}
	if err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 && want.Target == "" {
		t.Fatalf("notice %q should not have targets, got %d", title, targetCount)
	}
	if targetCount != 1 && want.Target != "" {
		t.Fatalf("notice %q target count=%d want 1 for %s", title, targetCount, want.Target)
	}
}
