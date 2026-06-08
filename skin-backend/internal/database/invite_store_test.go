package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestInviteStoreCreateListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.CreateInvite(ctx, "invite_a", 2, "Invite A"); err != nil {
		t.Fatal(err)
	}
	inv, err := db.GetInvite(ctx, "invite_a")
	if err != nil || inv == nil || inv.Code != "invite_a" || inv.TotalUses == nil || *inv.TotalUses != 2 || inv.UsedCount != 0 || inv.Note != "Invite A" {
		t.Fatalf("created invite mismatch: invite=%#v err=%v", inv, err)
	}
	list, err := db.ListInvites(ctx, 1, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["code"] != "invite_a" || items[0]["note"] != "Invite A" || list["has_next"] != false {
		t.Fatalf("invite list mismatch: %#v", list)
	}
	if err := db.DeleteInvite(ctx, "invite_a"); err != nil {
		t.Fatal(err)
	}
	if inv, err := db.GetInvite(ctx, "invite_a"); err != nil || inv != nil {
		t.Fatalf("invite should be deleted: invite=%#v err=%v", inv, err)
	}
}
