package permission

import (
	"context"
	"time"

	core "element-skin/backend/internal/permission"
)

const firstSuperAdminRoleLockID int64 = 0x5045524D53555052

func (s Store) GrantRole(ctx context.Context, userID, roleID, grantedBySubjectID string) error {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err := s.conn().Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (subject_id, role_id) DO UPDATE
		SET granted_by_subject_id=EXCLUDED.granted_by_subject_id
	`, SubjectIDForUser(userID), roleID, nullString(grantedBySubjectID), now)
		if err == nil && s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, SubjectIDForUser(userID))
	}
	return err
}

func (s Store) RevokeRole(ctx context.Context, userID, roleID string) (bool, error) {
	tag, err := s.conn().Exec(ctx, `
		DELETE FROM subject_roles
		WHERE subject_id=$1 AND role_id=$2
	`, SubjectIDForUser(userID), roleID)
	if err != nil {
		return false, err
	}
	affected := tag.RowsAffected() > 0
	if affected && s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, SubjectIDForUser(userID))
	}
	return affected, nil
}

func (s Store) GrantInitialSuperAdminIfNone(ctx context.Context, userID string) (bool, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return false, err
	}
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, firstSuperAdminRoleLockID); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subject_roles WHERE role_id=$1)`, core.RoleSuperAdmin).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, tx.Commit(ctx)
	}
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,created_at)
		VALUES ($1,$2,$3), ($1,$4,$3)
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, SubjectIDForUser(userID), core.RoleUser, now, core.RoleSuperAdmin); err != nil {
			if err == nil && s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, SubjectIDForUser(userID))
	}
	return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) RoleIDsForUser(ctx context.Context, userID string) ([]string, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.conn().Query(ctx, `
		SELECT role_id
		FROM subject_roles
		WHERE subject_id=$1
		ORDER BY role_id
	`, SubjectIDForUser(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		out = append(out, roleID)
	}
	return out, rows.Err()
}

func (s Store) UserHasRole(ctx context.Context, userID, roleID string) (bool, error) {
	var exists bool
	err := s.conn().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM subject_roles
			WHERE subject_id=$1 AND role_id=$2
		)
	`, SubjectIDForUser(userID), roleID).Scan(&exists)
	return exists, err
}

func (s Store) UserHasProtectedRole(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.conn().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM subject_roles sr
			JOIN roles r ON r.id=sr.role_id
			WHERE sr.subject_id=$1 AND r.protected=TRUE
		)
	`, SubjectIDForUser(userID)).Scan(&exists)
	return exists, err
}
