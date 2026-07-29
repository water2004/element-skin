package fallback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Endpoint struct {
	ID              int
	Priority        int
	SessionURL      string
	AccountURL      string
	ServicesURL     string
	CacheTTL        int
	SkinDomains     []string
	EnableProfile   bool
	EnableHasJoined bool
	EnableWhitelist bool
	Note            string
}

type Store struct {
	Pool *pgxpool.Pool
}

var (
	ErrEndpointNotFound  = errors.New("fallback endpoint not found")
	ErrDuplicateEndpoint = errors.New("duplicate fallback endpoint")
)

func IsEndpointNotFound(err error) bool {
	if errors.Is(err, ErrEndpointNotFound) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23503" &&
		pgErr.ConstraintName == "whitelisted_users_endpoint_id_fkey"
}

func (s Store) ListEndpoints(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT endpoint.id,endpoint.priority,endpoint.session_url,endpoint.account_url,
		       endpoint.services_url,endpoint.cache_ttl,
		       COALESCE(array_agg(domain.domain ORDER BY domain.sort_order,domain.domain)
		           FILTER (WHERE domain.domain IS NOT NULL), ARRAY[]::TEXT[]),
		       endpoint.enable_profile,endpoint.enable_hasjoined,endpoint.enable_whitelist,endpoint.note
		FROM fallback_endpoints endpoint
		LEFT JOIN fallback_skin_domains domain ON domain.endpoint_id=endpoint.id
		GROUP BY endpoint.id
		ORDER BY endpoint.priority,endpoint.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var e Endpoint
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

func (s Store) SaveEndpoints(ctx context.Context, endpoints []Endpoint) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := ReplaceEndpoints(ctx, tx, endpoints); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ReplaceEndpoints(ctx context.Context, tx pgx.Tx, endpoints []Endpoint) error {
	if _, err := tx.Exec(ctx, `LOCK TABLE fallback_endpoints IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	persistedIDs := make([]int, 0, len(endpoints))
	seenIDs := make(map[int]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		endpointID := endpoint.ID
		if endpointID > 0 {
			if _, exists := seenIDs[endpointID]; exists {
				return fmt.Errorf("%w: %d", ErrDuplicateEndpoint, endpointID)
			}
			seenIDs[endpointID] = struct{}{}
			command, err := tx.Exec(ctx, `
				UPDATE fallback_endpoints
				SET priority=$2,session_url=$3,account_url=$4,services_url=$5,cache_ttl=$6,
				    enable_profile=$7,enable_hasjoined=$8,enable_whitelist=$9,note=$10
				WHERE id=$1
			`, endpointID, endpoint.Priority, endpoint.SessionURL, endpoint.AccountURL,
				endpoint.ServicesURL, endpoint.CacheTTL, endpoint.EnableProfile,
				endpoint.EnableHasJoined, endpoint.EnableWhitelist, endpoint.Note)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return fmt.Errorf("%w: %d", ErrEndpointNotFound, endpointID)
			}
		} else {
			if err := tx.QueryRow(ctx, `
				INSERT INTO fallback_endpoints (
					priority,session_url,account_url,services_url,cache_ttl,
					enable_profile,enable_hasjoined,enable_whitelist,note
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
				RETURNING id
			`, endpoint.Priority, endpoint.SessionURL, endpoint.AccountURL, endpoint.ServicesURL,
				endpoint.CacheTTL, endpoint.EnableProfile, endpoint.EnableHasJoined,
				endpoint.EnableWhitelist, endpoint.Note).Scan(&endpointID); err != nil {
				return err
			}
		}
		persistedIDs = append(persistedIDs, endpointID)
		if _, err := tx.Exec(ctx, `DELETE FROM fallback_skin_domains WHERE endpoint_id=$1`, endpointID); err != nil {
			return err
		}
		for index, domain := range endpoint.SkinDomains {
			if _, err := tx.Exec(ctx, `
				INSERT INTO fallback_skin_domains (endpoint_id,domain,sort_order)
				VALUES ($1,$2,$3)
			`, endpointID, domain, index+1); err != nil {
				return err
			}
		}
	}
	_, err := tx.Exec(ctx, `DELETE FROM fallback_endpoints WHERE NOT (id = ANY($1::INTEGER[]))`, persistedIDs)
	return err
}

func (s Store) CollectSkinDomains(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT domain.domain
		FROM fallback_skin_domains domain
		JOIN fallback_endpoints endpoint ON endpoint.id=domain.endpoint_id
		ORDER BY endpoint.priority,endpoint.id,domain.sort_order,domain.domain
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		if !seen[domain] {
			seen[domain] = true
			out = append(out, domain)
		}
	}
	return out, rows.Err()
}

func (s Store) PrimaryEndpoint(ctx context.Context) (map[string]any, error) {
	eps, err := s.ListEndpoints(ctx)
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	return eps[0], nil
}

func (s Store) AddWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO whitelisted_users (username,endpoint_id,created_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, username, endpointID, time.Now().UnixMilli())
	return err
}

func (s Store) IsUserInWhitelist(ctx context.Context, username string, endpointID int) (bool, error) {
	var one int
	err := s.Pool.QueryRow(ctx, `SELECT 1 FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) ListWhitelistUsers(ctx context.Context, endpointID int) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `SELECT username,created_at FROM whitelisted_users WHERE endpoint_id=$1 ORDER BY username`, endpointID)
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

func (s Store) RemoveWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID)
	return err
}
