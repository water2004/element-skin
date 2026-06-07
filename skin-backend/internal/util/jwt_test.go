package util

import "testing"

func TestValidateJWTSecretRejectsWeakValues(t *testing.T) {
	for _, secret := range []string{"", DefaultJWTSecret, ShippedPlaceholderJWTSecret, "x123"} {
		if err := ValidateJWTSecret(secret); err == nil {
			t.Fatalf("expected weak secret %q to be rejected", secret)
		}
	}
}

func TestValidateJWTSecretCountsBytes(t *testing.T) {
	if err := ValidateJWTSecret("密密密密密密密密密密"); err == nil {
		t.Fatal("expected 30-byte UTF-8 secret to be rejected")
	}
	if err := ValidateJWTSecret("密密密密密密密密密密密"); err != nil {
		t.Fatalf("expected 33-byte UTF-8 secret to pass: %v", err)
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	token, err := CreateAccessToken("abcdefghijklmnopqrstuvwxyz123456", "user-id", true, 3600_000_000_000)
	if err != nil {
		t.Fatal(err)
	}
	claims, ok := DecodeAccessToken("abcdefghijklmnopqrstuvwxyz123456", token)
	if !ok {
		t.Fatal("token did not decode")
	}
	if claims["sub"] != "user-id" || claims["is_admin"] != true {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRefreshTokenHashStable(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || hash == "" {
		t.Fatal("refresh token and hash should be non-empty")
	}
	if HashRefreshToken(raw) != hash {
		t.Fatal("hash mismatch")
	}
}
