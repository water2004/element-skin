package notice

import (
	"context"
	"errors"
	"strconv"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

type UserListOptions struct {
	UserID               string
	CanReadAdminAudience bool
	Type                 string
	Limit                int
	Now                  int64
	IncludeRead          bool
	LastPinned           *bool
	LastCreated          *int64
	LastID               string
}

type AdminListOptions struct {
	Type        string
	Status      string
	Limit       int
	Now         int64
	LastPinned  *bool
	LastCreated *int64
	LastID      string
}

func (s Store) Create(ctx context.Context, n model.Notice) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notices (
			id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.CreatedBy, n.CreatedAt, n.UpdatedAt)
	return err
}

func (s Store) Get(ctx context.Context, id string) (*model.Notice, error) {
	row := s.Pool.QueryRow(ctx, noticeSelectSQL()+` WHERE id=$1`, id)
	n, err := scanNotice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s Store) GetForUser(ctx context.Context, id, userID string) (*model.NoticeView, error) {
	row := s.Pool.QueryRow(ctx, noticeViewSelectSQL("$2")+` WHERE n.id=$1`, id, userID)
	n, err := scanNoticeView(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s Store) ListForUser(ctx context.Context, opts UserListOptions) (map[string]any, error) {
	actual := opts.Limit + 1
	args := []any{opts.UserID, opts.Now}
	where := `n.enabled=TRUE AND (n.starts_at IS NULL OR n.starts_at <= $2) AND (n.ends_at IS NULL OR n.ends_at > $2) AND (n.audience='users'`
	if opts.CanReadAdminAudience {
		where += ` OR n.audience='admins'`
	}
	where += `) AND r.dismissed_at IS NULL`
	if opts.Type != "" {
		args = append(args, opts.Type)
		where += ` AND n.type=$` + strconv.Itoa(len(args))
	}
	if !opts.IncludeRead {
		where += ` AND r.read_at IS NULL`
	}
	where = addCursorWhere(where, &args, "n.", opts.LastPinned, opts.LastCreated, opts.LastID)
	args = append(args, actual)
	q := noticeViewSelectSQL("$1") + ` WHERE ` + where + ` ORDER BY n.pinned DESC, n.created_at DESC, n.id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []model.NoticeView{}
	for rows.Next() {
		item, err := scanNoticeView(rows)
		if err != nil {
			return nil, err
		}
		got = append(got, item)
	}
	return noticeViewPage(got, opts.Limit), rows.Err()
}

func (s Store) ListForAdmin(ctx context.Context, opts AdminListOptions) (map[string]any, error) {
	actual := opts.Limit + 1
	args := []any{}
	where := `TRUE`
	if opts.Type != "" {
		args = append(args, opts.Type)
		where += ` AND type=$` + strconv.Itoa(len(args))
	}
	switch opts.Status {
	case "enabled":
		where += ` AND enabled=TRUE`
	case "disabled":
		where += ` AND enabled=FALSE`
	case "expired":
		args = append(args, opts.Now)
		where += ` AND ends_at IS NOT NULL AND ends_at <= $` + strconv.Itoa(len(args))
	case "scheduled":
		args = append(args, opts.Now)
		where += ` AND starts_at IS NOT NULL AND starts_at > $` + strconv.Itoa(len(args))
	}
	where = addCursorWhere(where, &args, "", opts.LastPinned, opts.LastCreated, opts.LastID)
	args = append(args, actual)
	q := noticeSelectSQL() + ` WHERE ` + where + ` ORDER BY pinned DESC, created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []model.Notice{}
	for rows.Next() {
		item, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		got = append(got, item)
	}
	return noticePage(got, opts.Limit), rows.Err()
}

func (s Store) Update(ctx context.Context, n model.Notice) (*model.Notice, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE notices
		SET type=$2,title=$3,summary=$4,content_markdown=$5,display_mode=$6,level=$7,
			link_text=$8,link_url=$9,audience=$10,enabled=$11,pinned=$12,dismissible=$13,
			starts_at=$14,ends_at=$15,updated_at=$16
		WHERE id=$1
		RETURNING id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.UpdatedAt)
	updated, err := scanNotice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s Store) Replace(ctx context.Context, oldID string, n model.Notice) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM notices WHERE id=$1`, oldID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notices (
			id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.CreatedBy, n.CreatedAt, n.UpdatedAt); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s Store) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM notices WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) MarkRead(ctx context.Context, noticeID, userID string, now int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notice_receipts (notice_id,user_id,read_at,created_at)
		VALUES ($1,$2,$3,$3)
		ON CONFLICT (notice_id,user_id)
		DO UPDATE SET read_at=COALESCE(notice_receipts.read_at, EXCLUDED.read_at)
	`, noticeID, userID, now)
	return err
}

func (s Store) Dismiss(ctx context.Context, noticeID, userID string, now int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notice_receipts (notice_id,user_id,dismissed_at,created_at)
		VALUES ($1,$2,$3,$3)
		ON CONFLICT (notice_id,user_id)
		DO UPDATE SET dismissed_at=COALESCE(notice_receipts.dismissed_at, EXCLUDED.dismissed_at)
	`, noticeID, userID, now)
	return err
}

func (s Store) DeleteExpired(ctx context.Context, cutoff int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM notices WHERE ends_at IS NOT NULL AND ends_at <= $1`, cutoff)
	return err
}

func noticeSelectSQL() string {
	return `SELECT id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
		audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at FROM notices`
}

func noticeViewSelectSQL(userParam string) string {
	return `SELECT n.id,n.type,n.title,n.summary,n.content_markdown,n.display_mode,n.level,n.link_text,n.link_url,
		n.audience,n.enabled,n.pinned,n.dismissible,n.starts_at,n.ends_at,n.created_by,n.created_at,n.updated_at,
		r.read_at,r.dismissed_at
		FROM notices n LEFT JOIN notice_receipts r ON r.notice_id=n.id AND r.user_id=` + userParam
}

type noticeScanner interface {
	Scan(...any) error
}

func scanNotice(row noticeScanner) (model.Notice, error) {
	var n model.Notice
	err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Summary, &n.ContentMarkdown, &n.DisplayMode, &n.Level, &n.LinkText, &n.LinkURL,
		&n.Audience, &n.Enabled, &n.Pinned, &n.Dismissible, &n.StartsAt, &n.EndsAt, &n.CreatedBy, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func scanNoticeView(row noticeScanner) (model.NoticeView, error) {
	var v model.NoticeView
	err := row.Scan(&v.ID, &v.Type, &v.Title, &v.Summary, &v.ContentMarkdown, &v.DisplayMode, &v.Level, &v.LinkText, &v.LinkURL,
		&v.Audience, &v.Enabled, &v.Pinned, &v.Dismissible, &v.StartsAt, &v.EndsAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.ReadAt, &v.DismissedAt)
	v.Read = v.ReadAt != nil
	return v, err
}

func addCursorWhere(where string, args *[]any, columnPrefix string, lastPinned *bool, lastCreated *int64, lastID string) string {
	if lastPinned == nil || lastCreated == nil || lastID == "" {
		return where
	}
	pinned := 0
	if *lastPinned {
		pinned = 1
	}
	*args = append(*args, pinned, *lastCreated, lastID)
	i := len(*args) - 2
	return where + ` AND ((CASE WHEN ` + columnPrefix + `pinned THEN 1 ELSE 0 END) < $` + strconv.Itoa(i) +
		` OR ((CASE WHEN ` + columnPrefix + `pinned THEN 1 ELSE 0 END) = $` + strconv.Itoa(i) +
		` AND (` + columnPrefix + `created_at < $` + strconv.Itoa(i+1) +
		` OR (` + columnPrefix + `created_at = $` + strconv.Itoa(i+1) + ` AND ` + columnPrefix + `id < $` + strconv.Itoa(i+2) + `))))`
}

func noticePage(items []model.Notice, limit int) map[string]any {
	hasNext := len(items) > limit
	page := items
	if hasNext {
		page = items[:limit]
	}
	next := noticeCursor(page, hasNext)
	return map[string]any{"items": page, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(page)}
}

func noticeViewPage(items []model.NoticeView, limit int) map[string]any {
	hasNext := len(items) > limit
	page := items
	if hasNext {
		page = items[:limit]
	}
	next := noticeViewCursor(page, hasNext)
	return map[string]any{"items": page, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(page)}
}

func noticeCursor(items []model.Notice, hasNext bool) map[string]any {
	if !hasNext || len(items) == 0 {
		return nil
	}
	last := items[len(items)-1]
	return map[string]any{"last_pinned": last.Pinned, "last_created_at": last.CreatedAt, "last_id": last.ID}
}

func noticeViewCursor(items []model.NoticeView, hasNext bool) map[string]any {
	if !hasNext || len(items) == 0 {
		return nil
	}
	last := items[len(items)-1]
	return map[string]any{"last_pinned": last.Pinned, "last_created_at": last.CreatedAt, "last_id": last.ID}
}
