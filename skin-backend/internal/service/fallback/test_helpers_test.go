package fallback_test

import (
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service/fallback"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func newFallback(db *database.DB, client *http.Client) fallback.Fallback {
	redis := testutil.NewMemoryRedis()
	return fallback.Fallback{DB: db, Client: client, Redis: redis, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}
