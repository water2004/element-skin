package database

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
)

func (db *DB) CreateProfile(ctx context.Context, p model.Profile) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash)
	return err
}

func (db *DB) GetProfileByID(ctx context.Context, id string) (*model.Profile, error) {
	p, err := scanProfile(db.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE id=$1`, id))
	if IsNoRows(err) {
		return nil, nil
	}
	return &p, err
}

func (db *DB) GetProfileByName(ctx context.Context, name string) (*model.Profile, error) {
	p, err := scanProfile(db.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name=$1`, name))
	if IsNoRows(err) {
		return nil, nil
	}
	return &p, err
}

func (db *DB) GetProfilesByUser(ctx context.Context, userID string, limit int) ([]model.Profile, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) ListProfilesByUser(ctx context.Context, userID string, limit int, lastID string) (map[string]any, error) {
	actual := limit + 1
	var rows pgxRowsCompat
	var err error
	if lastID != "" {
		rows, err = db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 AND id>$2 ORDER BY id LIMIT $3`, userID, lastID, actual)
	} else {
		rows, err = db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2`, userID, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		got = append(got, p)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	next := map[string]any(nil)
	if hasNext {
		next = map[string]any{"last_id": got[limit-1].ID}
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, ProfileSummary(p))
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}

type pgxRowsCompat interface {
	Close()
	Next() bool
	Scan(...any) error
	Err() error
}

func ProfileSummary(p model.Profile) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "model": p.TextureModel, "skin_hash": p.SkinHash, "cape_hash": p.CapeHash}
}

func (db *DB) VerifyProfileOwnership(ctx context.Context, userID, profileID string) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM profiles WHERE id=$1 AND user_id=$2`, profileID, userID).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) CountProfilesByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=$1`, userID).Scan(&n)
	return n, err
}

func (db *DB) UpdateProfileName(ctx context.Context, id, name string) (bool, error) {
	tag, err := db.Pool.Exec(ctx, `UPDATE profiles SET name=$1 WHERE id=$2`, name, id)
	return tag.RowsAffected() > 0, err
}

func (db *DB) UpdateProfileSkin(ctx context.Context, id string, hash *string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET skin_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdateProfileCape(ctx context.Context, id string, hash *string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET cape_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdateProfileModel(ctx context.Context, id, model string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE id=$2`, model, id)
	return err
}

func (db *DB) DeleteProfileCascade(ctx context.Context, id string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM tokens WHERE profile_id=$1`, id); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (db *DB) SearchProfilesByNames(ctx context.Context, names []string, limit int) ([]model.Profile, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name = ANY($1) LIMIT $2`, names, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) ListAllProfiles(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	actual := limit + 1
	var rows pgxRowsCompat
	var err error
	if query != "" {
		pat := "%" + query + "%"
		if lastID != "" {
			rows, err = db.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1) AND p.id>$2 ORDER BY p.id LIMIT $3`, pat, lastID, actual)
		} else {
			rows, err = db.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1) ORDER BY p.id LIMIT $2`, pat, actual)
		}
	} else if lastID != "" {
		rows, err = db.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE p.id>$1 ORDER BY p.id LIMIT $2`, lastID, actual)
	} else {
		rows, err = db.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id ORDER BY p.id LIMIT $1`, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []map[string]any
	for rows.Next() {
		var p model.Profile
		var ownerEmail, ownerName string
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash, &ownerEmail, &ownerName); err != nil {
			return nil, err
		}
		item := ProfileSummary(p)
		item["user_id"] = p.UserID
		item["owner_email"] = ownerEmail
		item["owner_display_name"] = ownerName
		got = append(got, item)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		next = map[string]any{"last_id": got[limit-1]["id"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "page_size": len(items)}, rows.Err()
}

func ProfileModelKey(item map[string]any) map[string]any {
	if v, ok := item["texture_model"]; ok {
		item["model"] = v
	}
	return item
}

func IsProfileNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}
