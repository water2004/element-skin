package shared

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type AuthFunc func(http.HandlerFunc, ...permission.Definition) http.HandlerFunc

type ctxKey string

const (
	actorKey ctxKey = "actor"
)

func WithActor(ctx context.Context, actor permission.Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

func WithActorPermissions(ctx context.Context, userID string, definitions ...permission.Definition) context.Context {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, def := range definitions {
		bits.Set(def.BitIndex)
	}
	return WithActor(ctx, permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	})
}

func CurrentActor(req *http.Request) permission.Actor {
	actor, _ := req.Context().Value(actorKey).(permission.Actor)
	return actor
}

func CurrentUserID(req *http.Request) string {
	return CurrentActor(req).UserID
}

func RequirePermission(req *http.Request, def permission.Definition) error {
	if CurrentActor(req).Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func AsMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func CursorCreatedHash(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	hash, hashOK := m[hashKey].(string)
	if !ok || !hashOK || hash == "" {
		return nil, "", errors.New("invalid cursor")
	}
	created := &value
	return created, hash, nil
}

func PublicBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func ValidPublicValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return true
	case float64:
		return x == 0 || x == 1
	case int:
		return x == 0 || x == 1
	case string:
		return x == "true" || x == "false" || x == "0" || x == "1"
	default:
		return false
	}
}

func ParseImportProfiles(raw any) ([]map[string]string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
	}
	if len(items) == 0 {
		return nil, util.HTTPError{Status: 400, Detail: "profiles cannot be empty"}
	}
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
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

func ValueOrAny(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func ParsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive int")
	}
	return n, nil
}

func BearerToken(req *http.Request) (string, bool) {
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

func FormBool(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
}

func DecodeJSON(req *http.Request, dst any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(dst)
}

func MultipartFileBytes(req *http.Request, field string, maxBytes int64) ([]byte, error) {
	file, _, err := req.FormFile(field)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "file is required"}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, util.HTTPError{Status: 400, Detail: "File too large"}
	}
	return data, nil
}
