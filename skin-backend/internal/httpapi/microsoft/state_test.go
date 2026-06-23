package microsoft

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestPopStateRequiresKindAndConsumesToken(t *testing.T) {
	states := redisstore.NewMemoryStore()
	h := Handler{states: states}
	if err := states.SetState(context.Background(), "token", map[string]any{"kind": stateKindProfile, "user_id": "user-id"}, time.Minute); err != nil {
		t.Fatal(err)
	}

	session, err := h.popState(context.Background(), "token", stateKindProfile, "invalid")
	if err != nil {
		t.Fatal(err)
	}
	if session["user_id"] != "user-id" {
		t.Fatalf("unexpected session: %#v", session)
	}
	if _, err := h.popState(context.Background(), "token", stateKindProfile, "invalid"); err == nil {
		t.Fatal("state token should be consumed")
	}
}

func TestRequireStateOwner(t *testing.T) {
	if err := requireStateOwner(map[string]any{"user_id": "owner"}, "owner", "nope"); err != nil {
		t.Fatal(err)
	}
	if err := requireStateOwner(map[string]any{"user_id": "owner"}, "other", "nope"); err == nil {
		t.Fatal("expected owner mismatch error")
	}
}
