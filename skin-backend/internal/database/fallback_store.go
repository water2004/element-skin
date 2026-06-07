package database

import (
	"context"
	"strings"
)

type FallbackEndpoint struct {
	ID              int
	Priority        int
	SessionURL      string
	AccountURL      string
	ServicesURL     string
	CacheTTL        int
	SkinDomains     string
	EnableProfile   bool
	EnableHasJoined bool
	EnableWhitelist bool
	Note            string
}

func (db *DB) ListFallbackEndpoints(ctx context.Context) ([]map[string]any, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id,priority,session_url,account_url,services_url,cache_ttl,skin_domains,enable_profile,enable_hasjoined,enable_whitelist,note FROM fallback_endpoints ORDER BY priority,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var e FallbackEndpoint
		if err := rows.Scan(&e.ID, &e.Priority, &e.SessionURL, &e.AccountURL, &e.ServicesURL, &e.CacheTTL, &e.SkinDomains, &e.EnableProfile, &e.EnableHasJoined, &e.EnableWhitelist, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": e.ID, "priority": e.Priority, "session_url": e.SessionURL, "account_url": e.AccountURL,
			"services_url": e.ServicesURL, "cache_ttl": e.CacheTTL, "skin_domains": e.SkinDomains,
			"enable_profile": e.EnableProfile, "enable_hasjoined": e.EnableHasJoined,
			"enable_whitelist": e.EnableWhitelist, "note": e.Note,
		})
	}
	return out, rows.Err()
}

func (db *DB) SaveFallbackEndpoints(ctx context.Context, endpoints []FallbackEndpoint) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM fallback_endpoints`); err != nil {
		return err
	}
	for _, e := range endpoints {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fallback_endpoints (priority,session_url,account_url,services_url,cache_ttl,skin_domains,enable_profile,enable_hasjoined,enable_whitelist,note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, e.Priority, e.SessionURL, e.AccountURL, e.ServicesURL, e.CacheTTL, e.SkinDomains, e.EnableProfile, e.EnableHasJoined, e.EnableWhitelist, e.Note); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (db *DB) CollectFallbackSkinDomains(ctx context.Context) ([]string, error) {
	rows, err := db.Pool.Query(ctx, `SELECT skin_domains FROM fallback_endpoints ORDER BY priority,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, part := range strings.Split(raw, ",") {
			d := strings.TrimSpace(part)
			if d != "" && !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out, rows.Err()
}

func (db *DB) GetPrimaryFallbackEndpoint(ctx context.Context) (map[string]any, error) {
	eps, err := db.ListFallbackEndpoints(ctx)
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	return eps[0], nil
}

func (db *DB) AddWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO whitelisted_users (username,endpoint_id,created_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, username, endpointID, NowMS())
	return err
}

func (db *DB) IsUserInWhitelist(ctx context.Context, username string, endpointID int) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) ListWhitelistUsers(ctx context.Context, endpointID int) ([]map[string]any, error) {
	rows, err := db.Pool.Query(ctx, `SELECT username,created_at FROM whitelisted_users WHERE endpoint_id=$1 ORDER BY username`, endpointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var username string
		var createdAt int64
		if err := rows.Scan(&username, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"username": username, "created_at": createdAt})
	}
	return out, rows.Err()
}

func (db *DB) RemoveWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID)
	return err
}
