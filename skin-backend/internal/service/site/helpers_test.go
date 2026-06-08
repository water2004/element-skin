package site

import (
	"strings"
	"testing"
)

func TestSiteHelpersExactValues(t *testing.T) {
	for _, email := range []string{"user@example.com", "a.b+c@example.co"} {
		if !validEmail(email) {
			t.Fatalf("validEmail(%q)=false", email)
		}
	}
	for _, email := range []string{"", "user", "user@example", "user@example.com\nbcc@test.com", "Name <user@example.com>"} {
		if validEmail(email) {
			t.Fatalf("validEmail(%q)=true", email)
		}
	}
	if got := TextureHashBytes([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("TextureHashBytes mismatch: %s", got)
	}
	for input, want := range map[int]string{0: "0", 1: "1", 10: "10", 12345: "12345"} {
		if got := strconvI(input); got != want {
			t.Fatalf("strconvI(%d)=%q want %q", input, got, want)
		}
	}
	code, err := randomVerificationCode(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 16 || strings.Trim(code, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		t.Fatalf("verification code should use exact alphabet, got %q", code)
	}
	m := map[string]any{"last_id": "abc"}
	if got := asCursorMap(m); got["last_id"] != "abc" {
		t.Fatalf("asCursorMap should return map value: %#v", got)
	}
	if got := asCursorMap(nil); got != nil {
		t.Fatalf("asCursorMap(nil)=%#v", got)
	}
}
