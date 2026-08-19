package profile

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) ListMyProfiles(ctx context.Context, actor permission.Actor, cursor string, limit int) (map[string]any, error) {
	if err := requireActorPermission(actor, profileReadOwnedPermission); err != nil {
		return nil, err
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Profiles.ListByUser(ctx, actor.UserID, limit, last)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Service) ListAllProfiles(ctx context.Context, actor permission.Actor, cursor string, limit int, query string) (map[string]any, error) {
	if err := requireActorPermission(actor, profileReadAnyPermission); err != nil {
		return nil, err
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Profiles.ListAll(ctx, limit, last, strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Service) ListProfilesByUser(ctx context.Context, actor permission.Actor, userID, cursor string, limit int) (map[string]any, error) {
	if err := requireActorPermission(actor, profileReadAnyPermission); err != nil {
		return nil, err
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	lastID := ""
	if m != nil {
		lastID, _ = m["last_id"].(string)
	}
	if cursor != "" && lastID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	res, err := s.DB.Profiles.ListByUser(ctx, userID, limit, lastID)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}
