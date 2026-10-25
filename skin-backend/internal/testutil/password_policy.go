package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type PasswordPolicyCase struct {
	Name           string   `json:"name"`
	Password       string   `json:"password"`
	StrongErrors   []string `json:"strong_errors"`
	BasicErrors    []string `json:"basic_errors"`
	EntrypointTest bool     `json:"entrypoint_test"`
}

func PasswordPolicyCases(t testing.TB) []PasswordPolicyCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(), "..", "testdata", "password-policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []PasswordPolicyCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}
