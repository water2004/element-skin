package database

import (
	"context"

	"element-skin/backend/internal/model"
)

func (db *DB) AddToken(ctx context.Context, t model.Token) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO tokens (access_token,client_token,user_id,profile_id,created_at) VALUES ($1,$2,$3,$4,$5)`,
		t.AccessToken, t.ClientToken, t.UserID, t.ProfileID, t.CreatedAt)
	return err
}

func (db *DB) GetToken(ctx context.Context, access string) (*model.Token, error) {
	var t model.Token
	err := db.Pool.QueryRow(ctx, `SELECT access_token,client_token,user_id,profile_id,created_at FROM tokens WHERE access_token=$1`, access).
		Scan(&t.AccessToken, &t.ClientToken, &t.UserID, &t.ProfileID, &t.CreatedAt)
	if IsNoRows(err) {
		return nil, nil
	}
	return &t, err
}

func (db *DB) DeleteToken(ctx context.Context, access string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM tokens WHERE access_token=$1`, access)
	return err
}

func (db *DB) DeleteTokensByUser(ctx context.Context, userID string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1`, userID)
	return err
}

func (db *DB) CleanupTokens(ctx context.Context, userID string, cutoff int64, keep int) error {
	if _, err := db.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1 AND created_at < $2`, userID, cutoff); err != nil {
		return err
	}
	_, err := db.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1 AND access_token NOT IN (SELECT access_token FROM tokens WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2)`, userID, keep)
	return err
}

func (db *DB) AddSession(ctx context.Context, s model.Session) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO sessions (server_id,access_token,ip,created_at) VALUES ($1,$2,$3,$4)`, s.ServerID, s.AccessToken, s.IP, s.CreatedAt)
	return err
}

func (db *DB) ReplaceSession(ctx context.Context, s model.Session) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE server_id=$1`, s.ServerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO sessions (server_id,access_token,ip,created_at) VALUES ($1,$2,$3,$4)`, s.ServerID, s.AccessToken, s.IP, s.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) GetSession(ctx context.Context, serverID string) (*model.Session, error) {
	var s model.Session
	err := db.Pool.QueryRow(ctx, `SELECT server_id,access_token,ip,created_at FROM sessions WHERE server_id=$1`, serverID).
		Scan(&s.ServerID, &s.AccessToken, &s.IP, &s.CreatedAt)
	if IsNoRows(err) {
		return nil, nil
	}
	return &s, err
}

func (db *DB) AddRefreshToken(ctx context.Context, hash, userID string, expiresAt, createdAt int64) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO site_refresh_tokens (token_hash,user_id,expires_at,created_at) VALUES ($1,$2,$3,$4)`, hash, userID, expiresAt, createdAt)
	return err
}

func (db *DB) ConsumeRefreshToken(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := db.Pool.QueryRow(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1 RETURNING token_hash,user_id,expires_at,created_at`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}

func (db *DB) DeleteRefreshToken(ctx context.Context, hash string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1`, hash)
	return err
}

func (db *DB) DeleteRefreshTokensByUser(ctx context.Context, userID string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, userID)
	return err
}

func (db *DB) DeleteExpiredRefreshTokens(ctx context.Context, cutoff int64) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE expires_at < $1`, cutoff)
	return err
}

func (db *DB) GetRefreshToken(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := db.Pool.QueryRow(ctx, `SELECT token_hash,user_id,expires_at,created_at FROM site_refresh_tokens WHERE token_hash=$1`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}
