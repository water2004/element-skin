package shared

import (
	"strings"

	"element-skin/backend/internal/util"
)

func ParseImportProfiles(raw any) ([]map[string]string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Object: "profile", Operation: "import", Reason: "invalid"}
	}
	if len(items) == 0 {
		return nil, util.HTTPError{Status: 400, Object: "profile", Operation: "import", Reason: "required"}
	}
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Object: "profile", Operation: "import", Reason: "invalid"}
		}
		out = append(out, map[string]string{
			"profile_id":   strings.TrimSpace(AsString(m["profile_id"])),
			"profile_name": strings.TrimSpace(AsString(m["profile_name"])),
		})
	}
	return out, nil
}

func AsString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
