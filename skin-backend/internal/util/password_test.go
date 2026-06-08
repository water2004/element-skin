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
	want := []string{"密码长度至少 8 位", "密码需包含大写字母", "密码需包含数字"}
	if len(errs) != len(want) {
		t.Fatalf("unexpected strong password errors: %#v", errs)
	}
	for i := range want {
		if errs[i] != want[i] {
			t.Fatalf("error %d got %q want %q; all=%#v", i, errs[i], want[i], errs)
		}
	}
	if joined := JoinPasswordErrors(errs); joined != "密码长度至少 8 位；密码需包含大写字母；密码需包含数字" {
		t.Fatalf("unexpected joined password errors: %q", joined)
	}
}
