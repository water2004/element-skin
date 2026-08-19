package util

import "testing"

func TestPasswordHashVerifyAndStrongPasswordMessagesExact(t *testing.T) {
	hash, err := HashPassword("GoodPass123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "GoodPass123" || !VerifyPassword("GoodPass123", hash) || VerifyPassword("WrongPass123", hash) {
		t.Fatalf("password hash/verify mismatch: hash=%q", hash)
	}
	errs := ValidateStrongPassword("short")
	want := []string{"min_length", "uppercase", "number"}
	if len(errs) != len(want) {
		t.Fatalf("unexpected strong password errors: %#v", errs)
	}
	for i := range want {
		if errs[i] != want[i] {
			t.Fatalf("error %d got %q want %q; all=%#v", i, errs[i], want[i], errs)
		}
	}
}
