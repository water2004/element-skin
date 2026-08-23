package settings

import (
	"fmt"

	"element-skin/backend/internal/util"
)

var allowedEasterEggs = map[string]bool{
	"spring-festival":       true,
	"april-fools":           true,
	"qingming":              true,
	"children-day":          true,
	"dragon-boat":           true,
	"minecraft-anniversary": true,
	"mid-autumn":            true,
	"halloween":             true,
	"christmas":             true,
}

func ValidateEasterEggs(raw any) ([]string, error) {
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]string); ok {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			return nil, util.HTTPError{Status: 400, Object: "easter_egg_setting", Operation: "configure", Reason: "invalid"}
		}
	}

	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		id := fmt.Sprint(item)
		if !allowedEasterEggs[id] {
			return nil, util.HTTPError{Status: 400, Object: "easter_egg", Operation: "validate", Reason: "invalid", Params: map[string]any{"id": id}}
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}
