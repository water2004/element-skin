package database_test

import (
	"strings"
	"testing"

	"element-skin/backend/internal/database"
)

func TestInitSQLContainsExpectedTablesConstraintsIndexesAndSeeds(t *testing.T) {
	required := []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE TABLE IF NOT EXISTS profiles",
		"email TEXT UNIQUE NOT NULL",
		"name TEXT UNIQUE NOT NULL",
		"PRIMARY KEY(user_id, hash, texture_type)",
		"PRIMARY KEY(skin_hash, texture_type)",
		"UNIQUE(username, endpoint_id)",
		"idx_profiles_user_id",
		"idx_site_refresh_expires",
		"('site_name', '皮肤站')",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range required {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
	}
}
