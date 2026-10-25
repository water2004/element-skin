package util_test

import (
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHashAndVerification(t *testing.T) {
	hash, err := util.HashPassword("GoodPass123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "GoodPass123" || !util.VerifyPassword("GoodPass123", hash) || util.VerifyPassword("WrongPass123", hash) {
		t.Fatalf("password hash/verify mismatch: hash=%q", hash)
	}
}

func TestPasswordPolicyReturnsExactErrors(t *testing.T) {
	for _, fixture := range testutil.PasswordPolicyCases(t) {
		for _, strong := range []bool{false, true} {
			name := "basic/"
			want := fixture.BasicErrors
			if strong {
				name = "strong/"
				want = fixture.StrongErrors
			}
			t.Run(name+fixture.Name, func(t *testing.T) {
				got := util.ValidatePassword(fixture.Password, strong)
				if !reflect.DeepEqual(append([]string{}, got...), want) {
					t.Fatalf("errors=%#v; want %#v", got, want)
				}
			})
		}
	}
}

func TestPasswordHashUTF8ByteBoundary(t *testing.T) {
	for _, fixture := range testutil.PasswordPolicyCases(t) {
		switch fixture.Name {
		case "72 UTF8 bytes":
			if len(fixture.Password) != util.MaxPasswordBytes {
				t.Fatal("invalid 72-byte fixture")
			}
			hash, err := util.HashPassword(fixture.Password)
			if err != nil || !util.VerifyPassword(fixture.Password, hash) {
				t.Fatalf("72-byte hash verification failed: err=%v", err)
			}
		case "73 UTF8 bytes":
			if len(fixture.Password) != util.MaxPasswordBytes+1 {
				t.Fatal("invalid 73-byte fixture")
			}
			hash, err := util.HashPassword(fixture.Password)
			if hash != "" || !errors.Is(err, bcrypt.ErrPasswordTooLong) {
				t.Fatalf("oversized hash=%q error=%v; want empty hash and ErrPasswordTooLong", hash, err)
			}
		}
	}
}
