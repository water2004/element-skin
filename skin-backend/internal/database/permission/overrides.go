package permission

import (
	"context"
	"errors"
	"time"

	core "element-skin/backend/internal/permission"
)

type SubjectPermissionOverride struct {
	PermissionID   core.ID
	PermissionCode string
	Effect         string
	CreatedAt      int64
}

func (s Store) SetSubjectPermissionOverride(ctx context.Context, userID string, def core.Definition, effect string, grantedBySubjectID string) error {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return err
	}
	return s.SetPermissionOverrideForSubject(ctx, SubjectIDForUser(userID), def, effect, grantedBySubjectID)
}

func (s Store) SetPermissionOverrideForSubject(ctx context.Context, subjectID string, def core.Definition, effect string, grantedBySubjectID string) error {
	if effect != "allow" && effect != "deny" {
		return errors.New("permission override effect must be allow or deny")
	}
	now := time.Now().UnixMilli()
	_, err := s.conn().Exec(ctx, `
		INSERT INTO subject_permission_overrides (subject_id,permission_id,effect,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (subject_id, permission_id) DO UPDATE
		SET effect=EXCLUDED.effect, granted_by_subject_id=EXCLUDED.granted_by_subject_id
	`, subjectID, int64(def.ID), effect, nullString(grantedBySubjectID), now)
	if err == nil && s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, subjectID)
	}
	return err
}

func (s Store) SubjectPermissionOverridesForUser(ctx context.Context, userID string) ([]SubjectPermissionOverride, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	return s.SubjectPermissionOverridesForSubject(ctx, SubjectIDForUser(userID))
}

func (s Store) SubjectPermissionOverridesForSubject(ctx context.Context, subjectID string) ([]SubjectPermissionOverride, error) {
	rows, err := s.conn().Query(ctx, `
		SELECT p.id,p.code,spo.effect,spo.created_at
		FROM subject_permission_overrides spo
		JOIN permissions p ON p.id=spo.permission_id
		WHERE spo.subject_id=$1
		ORDER BY p.code
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubjectPermissionOverride
	for rows.Next() {
		var item SubjectPermissionOverride
		var permissionID int64
		if err := rows.Scan(&permissionID, &item.PermissionCode, &item.Effect, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.PermissionID = core.ID(permissionID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ClearSubjectPermissionOverride(ctx context.Context, userID string, def core.Definition) (bool, error) {
	return s.ClearPermissionOverrideForSubject(ctx, SubjectIDForUser(userID), def)
}

func (s Store) ClearPermissionOverrideForSubject(ctx context.Context, subjectID string, def core.Definition) (bool, error) {
	tag, err := s.conn().Exec(ctx, `
		DELETE FROM subject_permission_overrides
		WHERE subject_id=$1 AND permission_id=$2
	`, subjectID, int64(def.ID))
	if err != nil {
		return false, err
	}
	affected := tag.RowsAffected() > 0
	if affected && s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, subjectID)
	}
	return affected, nil
}
