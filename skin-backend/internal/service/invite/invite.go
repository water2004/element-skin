package invite

import (
	"context"
	"math"
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	inviteReadPermission   = permission.MustDefinitionByCode("invite.read.any")
	inviteCreatePermission = permission.MustDefinitionByCode("invite.create.any")
	inviteDeletePermission = permission.MustDefinitionByCode("invite.delete.any")
)

type Service struct {
	DB *database.DB
}

type CreateInput struct {
	Code         string
	CodeSet      bool
	TotalUses    any
	TotalUsesSet bool
	Note         string
}

func (s Service) List(ctx context.Context, actor permission.Actor, cursor string, limit int) (map[string]any, error) {
	if err := requirePermission(actor, inviteReadPermission); err != nil {
		return nil, err
	}
	lastCreated, lastCode, err := cursorCreatedHash(cursor, "last_code")
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Invites.List(ctx, limit, lastCreated, lastCode)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Service) Create(ctx context.Context, actor permission.Actor, input CreateInput) (map[string]any, error) {
	if err := requirePermission(actor, inviteCreatePermission); err != nil {
		return nil, err
	}
	code := input.Code
	if code == "" && !input.CodeSet {
		id, err := util.GenerateUUIDNoDash()
		if err != nil {
			return nil, err
		}
		code = id + id[:8]
	}
	if code == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "invite", Operation: "validate", Reason: "required"}
	}
	defaultTotal := 1
	total := &defaultTotal
	if input.TotalUsesSet && input.TotalUses == nil {
		total = nil
	} else if input.TotalUses != nil {
		v, ok := input.TotalUses.(float64)
		if !ok || v < 1 || v != math.Trunc(v) || v > float64(math.MaxInt32) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "invite_usage_limit", Operation: "validate", Reason: "invalid"}
		}
		value := int(v)
		total = &value
	}
	if err := s.DB.Invites.Create(ctx, code, total, input.Note); err != nil {
		return nil, err
	}
	var responseTotal any
	if total != nil {
		responseTotal = *total
	}
	return map[string]any{"code": code, "total_uses": responseTotal, "note": input.Note}, nil
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, code string) error {
	if err := requirePermission(actor, inviteDeletePermission); err != nil {
		return err
	}
	return s.DB.Invites.Delete(ctx, code)
}

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}

func cursorCreatedHash(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	hash, hashOK := m[hashKey].(string)
	if !ok || !hashOK || hash == "" {
		return nil, "", util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	created := &value
	return created, hash, nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
