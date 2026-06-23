package microsoft

import (
	"context"
	"time"

	"element-skin/backend/internal/redisstore"
)

const (
	TestStateKindOAuth   = stateKindOAuth
	TestStateKindProfile = stateKindProfile
	TestStateKindImport  = stateKindImport
)

func SeedStateForTest(states redisstore.Store, token string, session map[string]any, ttl time.Duration) error {
	return states.SetState(context.Background(), token, session, ttl)
}

func PopStateForTest(states redisstore.Store, token string) map[string]any {
	session, _ := states.PopState(context.Background(), token)
	return session
}
