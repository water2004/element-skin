package migration

import (
	"context"
	"fmt"

	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

// MigrateProfileNames runs inside schema initialization's transaction. The lock
// excludes runtime writers until names, audit rows and the unique index commit.
func MigrateProfileNames(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `LOCK TABLE profiles IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		SELECT id,name FROM (
			SELECT id,name,row_number() OVER (PARTITION BY lower(name) ORDER BY id COLLATE "C") AS position
			FROM profiles
		) ranked WHERE position > 1 ORDER BY id COLLATE "C"
	`)
	if err != nil {
		return err
	}
	type conflict struct{ id, name string }
	var conflicts []conflict
	for rows.Next() {
		var item conflict
		if err := rows.Scan(&item.id, &item.name); err != nil {
			rows.Close()
			return err
		}
		conflicts = append(conflicts, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range conflicts {
		renamed := false
		// Match ImportService: original name, then _1 through _99, truncated
		// by the same shared helper to leave space within the 16-character limit.
		for attempt := 1; attempt < 100; attempt++ {
			candidate := util.ProfileNameCandidate(item.name, attempt)
			var occupied bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE lower(name)=lower($1))`, candidate).Scan(&occupied); err != nil {
				return err
			}
			if occupied {
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE profiles SET name=$1 WHERE id=$2`, candidate, item.id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO profile_name_case_migrations (profile_id,old_name,new_name,migrated_at)
				VALUES ($1,$2,$3,(EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::BIGINT)
			`, item.id, item.name, candidate); err != nil {
				return err
			}
			renamed = true
			break
		}
		if !renamed {
			return fmt.Errorf("profile name migration exhausted import name candidates for profile %s; see doc/minecraft-services.md", item.id)
		}
	}
	_, err = tx.Exec(ctx, `
		ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_name_key;
		CREATE UNIQUE INDEX IF NOT EXISTS profiles_name_key ON profiles (lower(name));
	`)
	return err
}
