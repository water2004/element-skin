package integration_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

type passwordEntryPoint struct {
	name           string
	path           string
	passwordField  string
	successStatus  int
	emptyErrorBody string
}

var passwordEntryPoints = []passwordEntryPoint{
	{"register", "/v2/auth/register", "password", 201, "{\"error\":{\"object\":\"password\",\"operation\":\"validate\",\"reason\":\"required\"}}\n"},
	{"email_reset", "/v2/auth/password/reset", "password", 204, "{\"error\":{\"object\":\"registration\",\"operation\":\"validate\",\"reason\":\"required\"}}\n"},
	{"self", "/v2/users/me/password", "new_password", 204, "{\"error\":{\"object\":\"password\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"rules\":[\"required\"]}}}\n"},
	{"admin", "/v2/admin/users/password/reset", "new_password", 204, "{\"error\":{\"object\":\"password_reset\",\"operation\":\"validate\",\"reason\":\"required\"}}\n"},
}

func passwordPolicyErrorBody(t *testing.T, rules []string) string {
	t.Helper()
	encoded, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("{\"error\":{\"object\":\"password\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"rules\":%s}}}\n", encoded)
}
