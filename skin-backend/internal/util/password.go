package util

import (
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidateStrongPassword(password string) []string {
	var errs []string
	if len([]rune(password)) < 8 {
		errs = append(errs, "密码长度至少 8 位")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		errs = append(errs, "密码需包含小写字母")
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		errs = append(errs, "密码需包含大写字母")
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		errs = append(errs, "密码需包含数字")
	}
	return errs
}

func JoinPasswordErrors(errs []string) string {
	return strings.Join(errs, "；")
}
