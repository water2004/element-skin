package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Store struct {
	Pool *pgxpool.Pool
	q    Querier
}

func (s Store) conn() Querier {
	if s.q != nil {
		return s.q
	}
	return s.Pool
}

type policyKey struct {
	SessionKind string
	Entrypoint  string
}

var cachedSessionPolicies = buildSessionPolicyCache()

func buildSessionPolicyCache() map[policyKey]core.BitSet {
	policies := make(map[policyKey]core.BitSet, len(core.SessionPolicies))
	for _, sp := range core.SessionPolicies {
		key := policyKey{SessionKind: sp.SessionKind, Entrypoint: sp.Entrypoint}
		bits := core.NewBitSet(len(core.Definitions))
		for _, def := range sp.Permissions {
			bits.Set(def.BitIndex)
		}
		policies[key] = bits
	}
	return policies
}

func (s Store) cachedSessionPolicy(sessionKind, entrypoint string) (core.BitSet, bool) {
	bits, ok := cachedSessionPolicies[policyKey{SessionKind: sessionKind, Entrypoint: entrypoint}]
	return bits, ok
}
