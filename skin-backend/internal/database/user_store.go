package database

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE email=$1`, email))
	if IsNoRows(err) {
		return nil, nil
	}
	return &u, err
}

func (db *DB) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE id=$1`, id))
	if IsNoRows(err) {
		return nil, nil
	}
	return &u, err
}

func (db *DB) CreateUser(ctx context.Context, u model.User) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash)
	return err
}

func (db *DB) CreateUserWithProfile(ctx context.Context, u model.User, p model.Profile, inviteCode, usedBy string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash); err != nil {
		return err
	}
	if inviteCode != "" {
		tag, err := tx.Exec(ctx, `UPDATE invites SET used_count=used_count+1 WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)`, inviteCode)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInviteExhausted
		}
		if usedBy != "" {
			if _, err := tx.Exec(ctx, `UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL`, usedBy, inviteCode); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

var ErrInviteExhausted = errString("invite exhausted")

type errString string

func (e errString) Error() string { return string(e) }

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (db *DB) IsDisplayNameTaken(ctx context.Context, name string, exclude string) (bool, error) {
	var one int
	var err error
	if exclude != "" {
		err = db.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1 AND id<>$2`, name, exclude).Scan(&one)
	} else {
		err = db.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1`, name).Scan(&one)
	}
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) UpdateUser(ctx context.Context, id string, fields map[string]any) error {
	for k, v := range fields {
		switch k {
		case "email":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET email=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "display_name":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET display_name=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "preferred_language":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET preferred_language=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "avatar_hash":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET avatar_hash=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdatePasswordAndRevokeRefresh(ctx context.Context, id, hash string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (db *DB) DeleteUser(ctx context.Context, id string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`DELETE FROM profiles WHERE user_id=$1`,
		`DELETE FROM tokens WHERE user_id=$1`,
		`DELETE FROM site_refresh_tokens WHERE user_id=$1`,
		`DELETE FROM user_textures WHERE user_id=$1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return false, err
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (db *DB) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	var cur bool
	if err := db.Pool.QueryRow(ctx, `SELECT is_admin FROM users WHERE id=$1`, id).Scan(&cur); err != nil {
		return false, err
	}
	next := !cur
	_, err := db.Pool.Exec(ctx, `UPDATE users SET is_admin=$1 WHERE id=$2`, next, id)
	return next, err
}

func (db *DB) BanUser(ctx context.Context, id string, until int64) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	return err
}

func (db *DB) UnbanUser(ctx context.Context, id string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	return err
}

func (db *DB) IsBanned(ctx context.Context, id string) (bool, error) {
	var until *int64
	err := db.Pool.QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, id).Scan(&until)
	if IsNoRows(err) || until == nil {
		return false, nil
	}
	return *until > NowMS(), err
}

func (db *DB) CreateInvite(ctx context.Context, code string, totalUses int, note string) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO invites (code,created_at,total_uses,used_count,note) VALUES ($1,$2,$3,0,$4)`, code, NowMS(), totalUses, note)
	return err
}

func (db *DB) GetInvite(ctx context.Context, code string) (*model.Invite, error) {
	var inv model.Invite
	err := db.Pool.QueryRow(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites WHERE code=$1`, code).
		Scan(&inv.Code, &inv.CreatedAt, &inv.UsedBy, &inv.TotalUses, &inv.UsedCount, &inv.Note)
	if IsNoRows(err) {
		return nil, nil
	}
	return &inv, err
}

func (db *DB) DeleteInvite(ctx context.Context, code string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM invites WHERE code=$1`, code)
	return err
}

func (db *DB) ListInvites(ctx context.Context, limit int, lastCreated *int64, lastCode string) (map[string]any, error) {
	actual := limit + 1
	var rows pgx.Rows
	var err error
	if lastCreated != nil && lastCode != "" {
		rows, err = db.Pool.Query(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites WHERE (created_at < $1) OR (created_at=$1 AND code < $2) ORDER BY created_at DESC, code DESC LIMIT $3`, *lastCreated, lastCode, actual)
	} else {
		rows, err = db.Pool.Query(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites ORDER BY created_at DESC, code DESC LIMIT $1`, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []model.Invite
	for rows.Next() {
		var inv model.Invite
		if err := rows.Scan(&inv.Code, &inv.CreatedAt, &inv.UsedBy, &inv.TotalUses, &inv.UsedCount, &inv.Note); err != nil {
			return nil, err
		}
		got = append(got, inv)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, inv := range items {
		out = append(out, map[string]any{"code": inv.Code, "created_at": inv.CreatedAt, "used_by": inv.UsedBy, "total_uses": inv.TotalUses, "used_count": inv.UsedCount, "note": inv.Note})
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": *last.CreatedAt, "last_code": last.Code}
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}

func (db *DB) ListUsers(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	actual := limit + 1
	var rowsRows []model.User
	var rows pgx.Rows
	var err error
	if query != "" {
		pat := "%" + query + "%"
		if lastID != "" {
			rows, err = db.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE (display_name ILIKE $1 OR email ILIKE $1 OR EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id=users.id AND profiles.name ILIKE $1)) AND id>$2 ORDER BY id LIMIT $3`, pat, lastID, actual)
		} else {
			rows, err = db.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE (display_name ILIKE $1 OR email ILIKE $1 OR EXISTS (SELECT 1 FROM profiles WHERE profiles.user_id=users.id AND profiles.name ILIKE $1)) ORDER BY id LIMIT $2`, pat, actual)
		}
	} else if lastID != "" {
		rows, err = db.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE id>$1 ORDER BY id LIMIT $2`, lastID, actual)
	} else {
		rows, err = db.Pool.Query(ctx, `SELECT id,email,'' AS password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users ORDER BY id LIMIT $1`, actual)
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
	next := map[string]any(nil)
	if hasNext {
		next = map[string]any{"last_id": rowsRows[limit-1].ID}
	}
	out := make([]map[string]any, 0, len(items))
	for _, u := range items {
		out = append(out, map[string]any{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "is_admin": u.IsAdmin, "banned_until": u.BannedUntil, "preferred_language": u.PreferredLanguage, "avatar_hash": u.AvatarHash})
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}

func NormalizeProfileModel(m string) string {
	if m == "slim" {
		return "slim"
	}
	return "default"
}
