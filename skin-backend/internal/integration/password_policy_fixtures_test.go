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

var passwordPolicySettings = []struct {
	name    string
	value   any
	enabled bool
	stored  string
}{
	{"disabled", false, false, "false"},
	{"enabled", true, true, "true"},
	{"disabled_again", false, false, "false"},
	{"enabled_again", true, true, "true"},
}

var rejectedPasswordSettingUpdates = []struct {
	name      string
	body      string
	admin     bool
	status    int
	errorBody string
}{
	{"permission_denied", `{"enable_strong_password_check":true}`, false, 403, "{\"error\":{\"object\":\"permission\",\"operation\":\"check\",\"reason\":\"denied\"}}\n"},
	{"malformed_json", `{"enable_strong_password_check":`, true, 400, "{\"error\":{\"object\":\"request\",\"operation\":\"decode\",\"reason\":\"invalid\"}}\n"},
	{"string_true", `{"enable_strong_password_check":"true","rate_limit_auth_attempts":99}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"string_false", `{"enable_strong_password_check":"false"}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"string_one", `{"enable_strong_password_check":"1"}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"number_one", `{"enable_strong_password_check":1}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"number_zero", `{"enable_strong_password_check":0}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"null", `{"enable_strong_password_check":null}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"array", `{"enable_strong_password_check":[]}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
	{"object", `{"enable_strong_password_check":{}}`, true, 400, "{\"error\":{\"object\":\"setting_value\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"expected_type\":\"boolean\",\"key\":\"enable_strong_password_check\"}}}\n"},
}

func passwordPolicyErrorBody(t *testing.T, rules []string) string {
	t.Helper()
	encoded, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("{\"error\":{\"object\":\"password\",\"operation\":\"validate\",\"reason\":\"invalid\",\"params\":{\"rules\":%s}}}\n", encoded)
}
