package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s Store) Update(ctx context.Context, id string, fields map[string]any) error {
	attempted := false
	for _, key := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		if _, ok := fields[key]; ok {
			attempted = true
			break
		}
	}
	if !attempted {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&one); err != nil {
		return err
	}
	if displayName, ok := fields["display_name"].(string); ok {
		if err := lockDisplayName(ctx, tx, displayName, id); err != nil {
			return err
		}
	}
	assignments := make([]string, 0, len(fields))
	arguments := make([]any, 0, len(fields)+1)
	for _, k := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		v, ok := fields[k]
		if !ok {
			continue
		}
		arguments = append(arguments, v)
		assignments = append(assignments, fmt.Sprintf("%s=$%d", k, len(arguments)))
	}
	arguments = append(arguments, id)
	tag, err := tx.Exec(ctx, fmt.Sprintf(
		"UPDATE users SET %s WHERE id=$%d",
		strings.Join(assignments, ","),
		len(arguments),
	), arguments...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
