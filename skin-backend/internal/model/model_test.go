package model

import "testing"

func TestModelStructsPreserveExactFields(t *testing.T) {
	bannedUntil := int64(123)
	avatar := "avatar"
	user := User{ID: "uid", Email: "email", Password: "hash", PreferredLanguage: "zh_CN", DisplayName: "User", BannedUntil: &bannedUntil, AvatarHash: &avatar}
	if user.ID != "uid" || user.Email != "email" || user.Password != "hash" || user.PreferredLanguage != "zh_CN" || user.DisplayName != "User" ||
		user.BannedUntil == nil || *user.BannedUntil != 123 || user.AvatarHash == nil || *user.AvatarHash != "avatar" {
		t.Fatalf("User fields mismatch: %#v", user)
	}

	skin := "skin"
	cape := "cape"
	profile := Profile{ID: "pid", UserID: "uid", Name: "Role", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape}
	if profile.ID != "pid" || profile.UserID != "uid" || profile.Name != "Role" || profile.TextureModel != "slim" ||
		profile.SkinHash == nil || *profile.SkinHash != "skin" || profile.CapeHash == nil || *profile.CapeHash != "cape" {
		t.Fatalf("Profile fields mismatch: %#v", profile)
	}

	tokenProfile := "pid"
	token := Token{AccessToken: "access", ClientToken: "client", UserID: "uid", ProfileID: &tokenProfile, CreatedAt: 456}
	if token.AccessToken != "access" || token.ClientToken != "client" || token.UserID != "uid" || token.ProfileID == nil || *token.ProfileID != "pid" || token.CreatedAt != 456 {
		t.Fatalf("Token fields mismatch: %#v", token)
	}

	ip := "127.0.0.1"
	session := Session{ServerID: "server", AccessToken: "access", IP: &ip, CreatedAt: 789}
	if session.ServerID != "server" || session.AccessToken != "access" || session.IP == nil || *session.IP != "127.0.0.1" || session.CreatedAt != 789 {
		t.Fatalf("Session fields mismatch: %#v", session)
	}

	createdAt := int64(101)
	usedBy := "email"
	totalUses := 3
	invite := Invite{Code: "code", CreatedAt: &createdAt, UsedBy: &usedBy, TotalUses: &totalUses, UsedCount: 2, Note: "note"}
	if invite.Code != "code" || invite.CreatedAt == nil || *invite.CreatedAt != 101 || invite.UsedBy == nil || *invite.UsedBy != "email" ||
		invite.TotalUses == nil || *invite.TotalUses != 3 || invite.UsedCount != 2 || invite.Note != "note" {
		t.Fatalf("Invite fields mismatch: %#v", invite)
	}

	oauthClient := OAuthClient{
		ID:          "client",
		OwnerUserID: "uid",
		Name:        "Client",
		Description: "Description",
		RedirectURI: "https://app.example/callback",
		WebsiteURL:  "https://app.example",
		ClientType:  "confidential",
		SecretHash:  "secret",
		Status:      "active",
		CreatedAt:   111,
		UpdatedAt:   222,
	}
	if oauthClient.ID != "client" || oauthClient.OwnerUserID != "uid" || oauthClient.Name != "Client" ||
		oauthClient.Description != "Description" || oauthClient.RedirectURI != "https://app.example/callback" ||
		oauthClient.WebsiteURL != "https://app.example" || oauthClient.ClientType != "confidential" ||
		oauthClient.SecretHash != "secret" || oauthClient.Status != "active" ||
		oauthClient.CreatedAt != 111 || oauthClient.UpdatedAt != 222 {
		t.Fatalf("OAuthClient fields mismatch: %#v", oauthClient)
	}

	revokedAt := int64(333)
	oauthGrant := OAuthGrant{ID: "grant", UserID: "uid", SubjectID: "user:uid", ClientID: "client", Status: "revoked", CreatedAt: 444, RevokedAt: &revokedAt}
	if oauthGrant.ID != "grant" || oauthGrant.UserID != "uid" || oauthGrant.SubjectID != "user:uid" ||
		oauthGrant.ClientID != "client" || oauthGrant.Status != "revoked" || oauthGrant.CreatedAt != 444 ||
		oauthGrant.RevokedAt == nil || *oauthGrant.RevokedAt != 333 {
		t.Fatalf("OAuthGrant fields mismatch: %#v", oauthGrant)
	}

	consumedAt := int64(555)
	oauthCode := OAuthAuthorizationCode{
		CodeHash:            "code",
		ClientID:            "client",
		UserID:              "uid",
		GrantID:             "grant",
		RedirectURI:         "https://app.example/callback",
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           666,
		CreatedAt:           777,
		ConsumedAt:          &consumedAt,
	}
	if oauthCode.CodeHash != "code" || oauthCode.ClientID != "client" || oauthCode.UserID != "uid" ||
		oauthCode.GrantID != "grant" || oauthCode.RedirectURI != "https://app.example/callback" ||
		oauthCode.CodeChallenge != "challenge" || oauthCode.CodeChallengeMethod != "S256" ||
		oauthCode.ExpiresAt != 666 || oauthCode.CreatedAt != 777 ||
		oauthCode.ConsumedAt == nil || *oauthCode.ConsumedAt != 555 {
		t.Fatalf("OAuthAuthorizationCode fields mismatch: %#v", oauthCode)
	}

	tokenRevokedAt := int64(888)
	oauthToken := OAuthToken{TokenHash: "token", ClientID: "client", UserID: "uid", GrantID: "grant", ExpiresAt: 999, CreatedAt: 1000, RevokedAt: &tokenRevokedAt}
	if oauthToken.TokenHash != "token" || oauthToken.ClientID != "client" || oauthToken.UserID != "uid" ||
		oauthToken.GrantID != "grant" || oauthToken.ExpiresAt != 999 || oauthToken.CreatedAt != 1000 ||
		oauthToken.RevokedAt == nil || *oauthToken.RevokedAt != 888 {
		t.Fatalf("OAuthToken fields mismatch: %#v", oauthToken)
	}
}
