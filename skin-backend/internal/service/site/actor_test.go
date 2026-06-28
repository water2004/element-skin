package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func testActor(t testing.TB, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(context.Background(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("create test actor: %v", err)
	}
	return actor
}

func testActorWithCodes(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		bits.Set(def.BitIndex)
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func testUserActor(userID string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, role := range permission.Roles {
		if role.ID != permission.RoleUser {
			continue
		}
		for _, def := range role.Permissions {
			bits.Set(def.BitIndex)
		}
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}
