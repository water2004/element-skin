package permission

import (
	"context"
	"errors"
	"time"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

type EffectiveOptions struct {
	SessionKind       string
	Entrypoint        string
	DelegatedGrantID  string
	DelegatedClientID string
	ApplyBanPolicy    bool
}

func (s Store) EffectivePermissionsForUser(ctx context.Context, userID string, opts EffectiveOptions) (core.BitSet, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	subjectID := SubjectIDForUser(userID)
	permissions, err := s.effectivePermissionsForSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	if opts.SessionKind != "" || opts.Entrypoint != "" {
		policy, err := s.sessionPolicy(ctx, opts.SessionKind, opts.Entrypoint)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
	}
	if opts.DelegatedGrantID != "" {
		policy, err := s.delegationPolicy(ctx, userID, opts.DelegatedClientID, opts.DelegatedGrantID)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
	}
	if opts.ApplyBanPolicy {
		banned, err := s.userBanned(ctx, userID)
		if err != nil {
			return nil, err
		}
		if banned {
			join := core.MustDefinitionByCode("yggdrasil_server.join.bound_profile")
			permissions.Clear(join.BitIndex)
		}
	}
	return permissions, nil
}

func (s Store) ActorForUser(ctx context.Context, userID string, opts EffectiveOptions) (core.Actor, error) {
	permissions, err := s.EffectivePermissionsForUser(ctx, userID, opts)
	if err != nil {
		return core.Actor{}, err
	}
	return core.Actor{
		SubjectID:         SubjectIDForUser(userID),
		UserID:            userID,
		SessionKind:       opts.SessionKind,
		Entrypoint:        opts.Entrypoint,
		DelegationID:      opts.DelegatedGrantID,
		DelegatedClientID: opts.DelegatedClientID,
		Permissions:       permissions,
	}, nil
}

func (s Store) effectivePermissionsForSubject(ctx context.Context, subjectID string) (core.BitSet, error) {
	permissions := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM subject_roles sr
		JOIN role_permissions rp ON rp.role_id=sr.role_id
		JOIN permissions p ON p.id=rp.permission_id
		WHERE sr.subject_id=$1
		ORDER BY p.bit_index
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		permissions.Set(bitIndex)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = s.conn().Query(ctx, `
		SELECT p.bit_index, spo.effect
		FROM subject_permission_overrides spo
		JOIN permissions p ON p.id=spo.permission_id
		WHERE spo.subject_id=$1
		ORDER BY p.bit_index
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	denied := core.NewBitSet(len(core.Definitions))
	for rows.Next() {
		var bitIndex int
		var effect string
		if err := rows.Scan(&bitIndex, &effect); err != nil {
			return nil, err
		}
		if effect == "allow" {
			permissions.Set(bitIndex)
		} else {
			denied.Set(bitIndex)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions.AndNot(denied), nil
}

func (s Store) sessionPolicy(ctx context.Context, sessionKind, entrypoint string) (core.BitSet, error) {
	if cached, ok := s.cachedSessionPolicy(sessionKind, entrypoint); ok {
		return cached.Clone(), nil
	}
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM session_permission_policies spp
		JOIN permissions p ON p.id=spp.permission_id
		WHERE spp.session_kind=$1 AND spp.entrypoint=$2
		ORDER BY p.bit_index
	`, sessionKind, entrypoint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		policy.Set(bitIndex)
	}
	return policy, rows.Err()
}

func (s Store) delegationPolicy(ctx context.Context, userID, clientID, grantID string) (core.BitSet, error) {
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM delegated_permission_grants g
		JOIN delegated_clients c ON c.id=g.client_id
		JOIN delegated_grant_permissions gp ON gp.grant_id=g.id
		JOIN delegated_client_permissions cp ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
		JOIN permissions p ON p.id=gp.permission_id
		WHERE g.id=$1
		  AND g.user_id=$2
		  AND ($3='' OR g.client_id=$3)
		  AND g.status='active'
		  AND c.status='active'
		ORDER BY p.bit_index
	`, grantID, userID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		policy.Set(bitIndex)
	}
	return policy, rows.Err()
}

func (s Store) userBanned(ctx context.Context, userID string) (bool, error) {
	var bannedUntil *int64
	err := s.conn().QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, userID).Scan(&bannedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bannedUntil != nil && *bannedUntil > time.Now().UnixMilli(), nil
}
