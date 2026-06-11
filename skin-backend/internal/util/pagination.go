package util

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"strconv"
)

const DefaultLimit = 20
const MaxLimit = 100

func ClampLimit(v any, defaults ...int) int {
	def := DefaultLimit
	if len(defaults) > 0 {
		def = defaults[0]
	}
	var n int
	switch x := v.(type) {
	case nil:
		return def
	case int:
		n = x
	case string:
		parsed, err := strconv.Atoi(x)
		if err != nil {
			return def
		}
		n = parsed
	default:
		return def
	}
	if n < 1 {
		return 1
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

func EncodeCursor(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(cursor string) (map[string]any, error) {
	if cursor == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errors.New("cursor must be a JSON object")
	}
	return out, nil
}

func CursorInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, v >= 0
	case int:
		return int64(v), v >= 0
	case float64:
		if v < 0 || v > math.MaxInt64 || math.Trunc(v) != v {
			return 0, false
		}
		return int64(v), true
	default:
		return 0, false
	}
}
