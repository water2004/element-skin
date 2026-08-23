package invite_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	invitesvc "element-skin/backend/internal/service/invite"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestInviteServiceCreateListDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := invitesvc.Service{DB: db}
	actor := inviteActor("invite.read.any", "invite.create.any", "invite.delete.any")

	created, err := svc.Create(ctx, actor, invitesvc.CreateInput{Code: "service_invite", TotalUses: float64(2), Note: "Service Invite"})
	if err != nil {
		t.Fatal(err)
	}
	if created["code"] != "service_invite" || created["total_uses"] != 2 || created["note"] != "Service Invite" {
		t.Fatalf("created invite response mismatch: %#v", created)
	}
	row, err := db.Invites.Get(ctx, "service_invite")
	if err != nil || row == nil || row.TotalUses == nil || *row.TotalUses != 2 || row.Note != "Service Invite" || row.UsedCount != 0 {
		t.Fatalf("created invite row mismatch: row=%#v err=%v", row, err)
	}

	page, err := svc.List(ctx, actor, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := page["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("invite page items mismatch: %#v", page)
	}
	if items[0]["code"] != "service_invite" || items[0]["note"] != "Service Invite" || page["has_next"] != false || page["next_cursor"] != "" {
		t.Fatalf("invite page exact fields mismatch: %#v", page)
	}

	if err := svc.Delete(ctx, actor, "service_invite"); err != nil {
		t.Fatal(err)
	}
	if row, err := db.Invites.Get(ctx, "service_invite"); err != nil || row != nil {
		t.Fatalf("deleted invite should be absent: row=%#v err=%v", row, err)
	}
}

func TestInviteServiceGeneratedCodeAndCursorPaginationExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := invitesvc.Service{DB: db}
	actor := inviteActor("invite.read.any", "invite.create.any")

	generated, err := svc.Create(ctx, actor, invitesvc.CreateInput{Note: "Generated"})
	if err != nil {
		t.Fatal(err)
	}
	code, _ := generated["code"].(string)
	if len(code) != 40 || generated["total_uses"] != 1 || generated["note"] != "Generated" {
		t.Fatalf("generated invite mismatch: %#v", generated)
	}
	if row, err := db.Invites.Get(ctx, code); err != nil || row == nil || row.Code != code || row.TotalUses == nil || *row.TotalUses != 1 {
		t.Fatalf("generated invite row mismatch: row=%#v err=%v", row, err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE invites SET created_at=$1 WHERE code=$2`, int64(1000), code); err != nil {
		t.Fatal(err)
	}
	if err := db.Invites.Create(ctx, "service_invite_cursor_a", testutil.Pointer(1), "Cursor A"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE invites SET created_at=$1 WHERE code=$2`, int64(900), "service_invite_cursor_a"); err != nil {
		t.Fatal(err)
	}

	first, err := svc.List(ctx, actor, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	firstItems := first["items"].([]map[string]any)
	cursor, _ := first["next_cursor"].(string)
	if len(firstItems) != 1 || firstItems[0]["code"] != code || first["has_next"] != true || cursor == "" {
		t.Fatalf("first invite page mismatch: %#v", first)
	}
	second, err := svc.List(ctx, actor, cursor, 1)
	if err != nil {
		t.Fatal(err)
	}
	secondItems := second["items"].([]map[string]any)
	if len(secondItems) != 1 || secondItems[0]["code"] != "service_invite_cursor_a" || second["has_next"] != false || second["next_cursor"] != "" {
		t.Fatalf("second invite page mismatch: %#v", second)
	}
}

func TestInviteServiceDistinguishesOmittedAndUnlimitedTotalUses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := invitesvc.Service{DB: db}
	actor := inviteActor("invite.create.any")

	unlimited, err := svc.Create(ctx, actor, invitesvc.CreateInput{
		Code:         "service_unlimited_invite",
		TotalUsesSet: true,
		Note:         "Unlimited",
	})
	if err != nil {
		t.Fatal(err)
	}
	if unlimited["code"] != "service_unlimited_invite" || unlimited["total_uses"] != nil || unlimited["note"] != "Unlimited" {
		t.Fatalf("unlimited invite response mismatch: %#v", unlimited)
	}
	unlimitedRow, err := db.Invites.Get(ctx, "service_unlimited_invite")
	if err != nil || unlimitedRow == nil || unlimitedRow.TotalUses != nil || unlimitedRow.UsedCount != 0 {
		t.Fatalf("unlimited invite row mismatch: row=%#v err=%v", unlimitedRow, err)
	}

	defaulted, err := svc.Create(ctx, actor, invitesvc.CreateInput{Code: "service_default_invite"})
	if err != nil {
		t.Fatal(err)
	}
	if defaulted["total_uses"] != 1 {
		t.Fatalf("default invite response mismatch: %#v", defaulted)
	}
	defaultedRow, err := db.Invites.Get(ctx, "service_default_invite")
	if err != nil || defaultedRow == nil || defaultedRow.TotalUses == nil || *defaultedRow.TotalUses != 1 {
		t.Fatalf("default invite row mismatch: row=%#v err=%v", defaultedRow, err)
	}
}

func TestInviteServiceRejectsInvalidInputsAndPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := invitesvc.Service{DB: db}
	actor := inviteActor("invite.read.any", "invite.create.any", "invite.delete.any")

	if _, err := svc.List(ctx, permission.Actor{}, "", 10); !inviteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("List without permission mismatch: %#v", err)
	}
	if _, err := svc.Create(ctx, permission.Actor{}, invitesvc.CreateInput{Code: "denied"}); !inviteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("Create without permission mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, permission.Actor{}, "denied"); !inviteHTTPError(err, http.StatusForbidden, "permission.check.denied") {
		t.Fatalf("Delete without permission mismatch: %#v", err)
	}
	if _, err := svc.List(ctx, actor, "not-a-cursor", 10); !inviteHTTPError(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("invalid cursor mismatch: %#v", err)
	}
	if _, err := svc.List(ctx, actor, util.EncodeCursor(map[string]any{"last_created_at": database.NowMS()}), 10); !inviteHTTPError(err, http.StatusBadRequest, "pagination_cursor.decode.invalid") {
		t.Fatalf("incomplete cursor mismatch: %#v", err)
	}
	if _, err := svc.Create(ctx, actor, invitesvc.CreateInput{CodeSet: true}); !inviteHTTPError(err, http.StatusBadRequest, "invite.validate.required") {
		t.Fatalf("explicit empty code mismatch: %#v", err)
	}
	oneCharacter, err := svc.Create(ctx, actor, invitesvc.CreateInput{Code: "引", CodeSet: true})
	if err != nil || oneCharacter["code"] != "引" || oneCharacter["total_uses"] != 1 {
		t.Fatalf("single-character Unicode code mismatch: created=%#v err=%v", oneCharacter, err)
	}
	for _, totalUses := range []any{"2", float64(0), float64(1.5), float64(2147483648)} {
		if _, err := svc.Create(ctx, actor, invitesvc.CreateInput{Code: "bad_total_" + util.StripUUIDDashes("00000000-0000-0000-0000-000000000000"), TotalUses: totalUses}); !inviteHTTPError(err, http.StatusBadRequest, "invite_usage_limit.validate.invalid") {
			t.Fatalf("invalid total_uses=%#v mismatch: %#v", totalUses, err)
		}
	}
	if row, err := db.Invites.Get(ctx, "引"); err != nil || row == nil || row.Code != "引" {
		t.Fatalf("arbitrary non-empty invite must be persisted: row=%#v err=%v", row, err)
	}

	db.Close()
	if created, err := svc.Create(ctx, actor, invitesvc.CreateInput{}); created != nil || err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("Create generated invite on closed database = created=%#v err=%v; want nil closed pool", created, err)
	}
}

func inviteActor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "invite-test",
		UserID:      "invite-test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func inviteHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Error() == detail
}
