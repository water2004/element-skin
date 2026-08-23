package util

import (
	"regexp"

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
		errs = append(errs, "min_length")
	}
	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		errs = append(errs, "lowercase")
	}
	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		errs = append(errs, "uppercase")
	}
	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		errs = append(errs, "number")
	}
	return errs
}
