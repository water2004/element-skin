package profile

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func NormalizeModel(m string) string {
	if m == "slim" {
		return "slim"
	}
	return "default"
}

func Summary(p model.Profile) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "model": p.TextureModel, "skin_hash": p.SkinHash, "cape_hash": p.CapeHash}
}

func ModelKey(item map[string]any) map[string]any {
	if v, ok := item["texture_model"]; ok {
		item["model"] = v
	}
	return item
}

func IsNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

func (s Store) Create(ctx context.Context, p model.Profile) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash)
	return err
}

func (s Store) GetByID(ctx context.Context, id string) (*model.Profile, error) {
	p, err := scan(s.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

func (s Store) GetByName(ctx context.Context, name string) (*model.Profile, error) {
	p, err := scan(s.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name=$1`, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

func (s Store) GetByUser(ctx context.Context, userID string, limit int) ([]model.Profile, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2`, userID, limit)
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

func (s Store) VerifyOwnership(ctx context.Context, userID, profileID string) (bool, error) {
	var one int
	err := s.Pool.QueryRow(ctx, `SELECT 1 FROM profiles WHERE id=$1 AND user_id=$2`, profileID, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) CountByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=$1`, userID).Scan(&n)
	return n, err
}

func (s Store) UpdateName(ctx context.Context, id, name string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE profiles SET name=$1 WHERE id=$2`, name, id)
	return tag.RowsAffected() > 0, err
}

func (s Store) UpdateSkin(ctx context.Context, id string, hash *string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE profiles SET skin_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (s Store) UpdateSkinAndModel(ctx context.Context, id string, hash *string, model string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE profiles SET skin_hash=$1,texture_model=$2 WHERE id=$3`, hash, model, id)
	return err
}

func (s Store) UpdateCape(ctx context.Context, id string, hash *string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE profiles SET cape_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (s Store) UpdateModel(ctx context.Context, id, model string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE id=$2`, model, id)
	return err
}

func (s Store) DeleteCascade(ctx context.Context, id string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (s Store) SearchByNames(ctx context.Context, names []string, limit int) ([]model.Profile, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name = ANY($1) LIMIT $2`, names, limit)
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

func scan(row pgx.Row) (model.Profile, error) {
	var p model.Profile
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash)
	return p, err
}
