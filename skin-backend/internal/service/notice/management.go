package notice

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	noticedb "element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) ListForManagement(ctx context.Context, actor permission.Actor, params ListParams) (map[string]any, error) {
	if err := requirePermission(actor, noticeReadPermission); err != nil {
		return nil, err
	}
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = StatusAll
	}
	if !validStatus(status) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "status", Operation: "validate", Reason: "invalid"}
	}
	typ := strings.TrimSpace(params.Type)
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "type", Operation: "validate", Reason: "invalid"}
	}
	return s.DB.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{
		Type:        typ,
		Status:      status,
		Limit:       params.Limit,
		Now:         database.NowMS(),
		LastPinned:  cur.lastPinned,
		LastCreated: cur.lastCreated,
		LastID:      cur.lastID,
	})
}

func (s Service) DeleteExpired(ctx context.Context, actor permission.Actor, cutoff int64) error {
	if err := requirePermission(actor, noticeDeleteSystemPermission); err != nil {
		return err
	}
	return s.DB.Notices.DeleteExpired(ctx, cutoff)
}
