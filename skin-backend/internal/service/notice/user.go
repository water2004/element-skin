package notice

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	noticedb "element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) ListForUser(ctx context.Context, actor permission.Actor, params ListParams) (map[string]any, error) {
	if err := requirePermission(actor, noticeReadOwnedPermission); err != nil {
		return nil, err
	}
	user := noticeUser(actor)
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	typ := strings.TrimSpace(params.Type)
	if params.Dashboard && typ == "" {
		typ = TypeAnnouncement
	}
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "type", Operation: "validate", Reason: "invalid"}
	}
	return s.DB.Notices.ListForUser(ctx, noticedb.UserListOptions{
		UserID:               user.ID,
		CanReadAdminAudience: user.CanReadAdminAudience,
		Type:                 typ,
		Limit:                params.Limit,
		Now:                  database.NowMS(),
		IncludeRead:          params.IncludeRead || params.Dashboard,
		LastPinned:           cur.lastPinned,
		LastCreated:          cur.lastCreated,
		LastID:               cur.lastID,
	})
}

func (s Service) GetForUser(ctx context.Context, id string, actor permission.Actor) (*model.NoticeView, error) {
	if err := requirePermission(actor, noticeReadOwnedPermission); err != nil {
		return nil, err
	}
	user := noticeUser(actor)
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return nil, err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
	}
	now := database.NowMS()
	if err := s.DB.Notices.MarkRead(ctx, id, user.ID, now); err != nil {
		return nil, err
	}
	if item.ReadAt == nil {
		item.ReadAt = &now
		item.Read = true
	}
	return item, nil
}

func (s Service) MarkRead(ctx context.Context, id string, actor permission.Actor) error {
	if err := requirePermission(actor, noticeReadOwnedPermission); err != nil {
		return err
	}
	user := noticeUser(actor)
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
	}
	return s.DB.Notices.MarkRead(ctx, id, user.ID, database.NowMS())
}

func (s Service) Dismiss(ctx context.Context, id string, actor permission.Actor) error {
	if err := requirePermission(actor, noticeDismissOwnedPermission); err != nil {
		return err
	}
	user := noticeUser(actor)
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Object: "notice", Operation: "resolve", Reason: "not_found"}
	}
	if !item.Dismissible {
		return util.HTTPError{Status: http.StatusForbidden, Object: "notice", Operation: "dismiss", Reason: "denied"}
	}
	return s.DB.Notices.Dismiss(ctx, id, user.ID, database.NowMS())
}
