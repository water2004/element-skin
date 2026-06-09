package microsoft

import (
	"time"

	"element-skin/backend/internal/util"
)

const (
	TestStateKindOAuth   = stateKindOAuth
	TestStateKindProfile = stateKindProfile
	TestStateKindImport  = stateKindImport
)

func SeedStateForTest(states *util.InMemoryStateStore, token string, session map[string]any, ttl time.Duration) {
	states.Put(token, session, ttl)
}

func PopStateForTest(states *util.InMemoryStateStore, token string) map[string]any {
	session, _ := states.Pop(token).(map[string]any)
	return session
}
