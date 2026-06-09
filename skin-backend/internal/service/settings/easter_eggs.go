package settings

import (
	"fmt"

	"element-skin/backend/internal/util"
)

var allowedEasterEggs = map[string]bool{
	"spring-festival": true,
	"april-fools":  true,
	"qingming":     true,
	"children-day": true,
	"dragon-boat":  true,
	"mid-autumn":   true,
	"christmas":    true,
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
			return nil, util.HTTPError{Status: 400, Detail: "invalid easter_eggs_enabled"}
		}
	}

	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		id := fmt.Sprint(item)
		if !allowedEasterEggs[id] {
			return nil, util.HTTPError{Status: 400, Detail: "invalid easter egg: " + id}
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}
