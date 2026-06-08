package user

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) List(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	actual := limit + 1
	var rowsRows []model.User
	var rows pgx.Rows
	var err error
	if query != "" {
		pat := "%" + query + "%"
		if lastID != "" {
			rows, err = s.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE (display_name ILIKE $1 OR email ILIKE $1 OR EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id=users.id AND profiles.name ILIKE $1)) AND id>$2 ORDER BY id LIMIT $3`, pat, lastID, actual)
		} else {
			rows, err = s.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE (display_name ILIKE $1 OR email ILIKE $1 OR EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id=users.id AND profiles.name ILIKE $1)) ORDER BY id LIMIT $2`, pat, actual)
		}
	} else if lastID != "" {
		rows, err = s.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE id>$1 ORDER BY id LIMIT $2`, lastID, actual)
	} else {
		rows, err = s.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users ORDER BY id LIMIT $1`, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Password, &u.IsAdmin, &u.PreferredLanguage, &u.DisplayName, &u.BannedUntil, &u.AvatarHash); err != nil {
			return nil, err
		}
		rowsRows = append(rowsRows, u)
	}
	hasNext := len(rowsRows) > limit
	items := rowsRows
	if hasNext {
		items = rowsRows[:limit]
	}
	var next any
	if hasNext {
		next = map[string]any{"last_id": rowsRows[limit-1].ID}
	}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		out = append(out, PublicUser(u))
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}
