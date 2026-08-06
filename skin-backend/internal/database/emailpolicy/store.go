package emailpolicy

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Get(ctx context.Context) (model.EmailSuffixPolicy, error) {
	policy := model.EmailSuffixPolicy{
		Mode:      model.EmailSuffixModeDisabled,
		Allowlist: []string{},
		Denylist:  []string{},
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT policy.mode,rule.list_type,rule.suffix
		FROM email_suffix_policy policy
		LEFT JOIN email_suffix_rules rule ON TRUE
		WHERE policy.singleton=TRUE
		ORDER BY rule.list_type,rule.suffix
	`)
	if err != nil {
		return model.EmailSuffixPolicy{}, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		found = true
		var listType, suffix *string
		if err := rows.Scan(&policy.Mode, &listType, &suffix); err != nil {
			return model.EmailSuffixPolicy{}, err
		}
		if listType == nil || suffix == nil {
			continue
		}
		switch *listType {
		case model.EmailSuffixModeAllowlist:
			policy.Allowlist = append(policy.Allowlist, *suffix)
		case model.EmailSuffixModeDenylist:
			policy.Denylist = append(policy.Denylist, *suffix)
		}
	}
	if err := rows.Err(); err != nil {
		return model.EmailSuffixPolicy{}, err
	}
	if !found {
		return model.EmailSuffixPolicy{}, pgx.ErrNoRows
	}
	return policy, nil
}

func (s Store) Replace(ctx context.Context, policy model.EmailSuffixPolicy) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var singleton bool
	if err := tx.QueryRow(ctx, `SELECT singleton FROM email_suffix_policy WHERE singleton=TRUE FOR UPDATE`).Scan(&singleton); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM email_suffix_rules`); err != nil {
		return err
	}
	for _, suffix := range policy.Allowlist {
		if _, err := tx.Exec(ctx, `INSERT INTO email_suffix_rules (list_type,suffix) VALUES ($1,$2)`, model.EmailSuffixModeAllowlist, suffix); err != nil {
			return err
		}
	}
	for _, suffix := range policy.Denylist {
		if _, err := tx.Exec(ctx, `INSERT INTO email_suffix_rules (list_type,suffix) VALUES ($1,$2)`, model.EmailSuffixModeDenylist, suffix); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE email_suffix_policy SET mode=$1 WHERE singleton=TRUE`, policy.Mode); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
