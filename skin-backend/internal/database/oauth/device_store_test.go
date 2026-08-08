package oauth_test

import (
	"context"
	"reflect"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestDeviceCodeLifecyclePreservesExactFieldsPermissionsAndStates(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device@test.com", "pw", "OAuthDevice", false)
	client := model.OAuthClient{
		ID:          "device-client",
		OwnerUserID: user.ID,
		Name:        "Device client",
		RedirectURI: "https://device.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	permissions := permissionIDs("account.read.self", "minecraft_profile.read.public")
	if err := db.OAuth.CreateClient(ctx, client, permissions, nil); err != nil {
		t.Fatal(err)
	}
	subjectID := permissiondb.SubjectIDForUser(user.ID)
	code := model.OAuthDeviceCode{
		DeviceCodeHash: "device-hash-1",
		UserCodeHash:   "user-hash-1",
		ClientID:       client.ID,
		Status:         "pending",
		ExpiresAt:      5000,
		CreatedAt:      1100,
	}
	if err := db.OAuth.CreateDeviceCode(ctx, code, permissions); err != nil {
		t.Fatal(err)
	}
	got, gotPermissions, err := db.OAuth.GetDeviceCodeByUserCodeHash(ctx, code.UserCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &code) {
		t.Fatalf("device code by user code mismatch:\n got=%#v\nwant=%#v", got, &code)
	}
	if !reflect.DeepEqual(gotPermissions, permissions) {
		t.Fatalf("device code permissions=%v want=%v", gotPermissions, permissions)
	}
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, code.UserCodeHash, user.ID, subjectID, 1200); err != nil || !ok {
		t.Fatalf("ApproveDeviceCode should approve pending code: ok=%v err=%v", ok, err)
	}
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, code.UserCodeHash, user.ID, subjectID, 1250); err != nil || ok {
		t.Fatalf("ApproveDeviceCode should reject non-pending code: ok=%v err=%v", ok, err)
	}
	got, _, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, code.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	approvedAt := int64(1200)
	wantApproved := code
	wantApproved.UserID = &user.ID
	wantApproved.SubjectID = &subjectID
	wantApproved.Status = "approved"
	wantApproved.ApprovedAt = &approvedAt
	if !reflect.DeepEqual(got, &wantApproved) {
		t.Fatalf("approved device code mismatch:\n got=%#v\nwant=%#v", got, &wantApproved)
	}
	if err := db.OAuth.MarkDeviceCodePolled(ctx, code.DeviceCodeHash, 1260); err != nil {
		t.Fatal(err)
	}
	got, _, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, code.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	lastPolledAt := int64(1260)
	wantApproved.LastPolledAt = &lastPolledAt
	if !reflect.DeepEqual(got, &wantApproved) {
		t.Fatalf("polled device code mismatch:\n got=%#v\nwant=%#v", got, &wantApproved)
	}
	consumed, consumedPermissions, err := db.OAuth.ConsumeApprovedDeviceCode(ctx, code.DeviceCodeHash, 1300)
	if err != nil {
		t.Fatal(err)
	}
	consumedAt := int64(1300)
	wantConsumed := wantApproved
	wantConsumed.Status = "consumed"
	wantConsumed.ConsumedAt = &consumedAt
	if !reflect.DeepEqual(consumed, &wantConsumed) {
		t.Fatalf("consumed device code mismatch:\n got=%#v\nwant=%#v", consumed, &wantConsumed)
	}
	if !reflect.DeepEqual(consumedPermissions, permissions) {
		t.Fatalf("consumed permissions=%v want=%v", consumedPermissions, permissions)
	}
	replay, replayPermissions, err := db.OAuth.ConsumeApprovedDeviceCode(ctx, code.DeviceCodeHash, 1400)
	if err != nil {
		t.Fatal(err)
	}
	if replay != nil || replayPermissions != nil {
		t.Fatalf("device code replay should return nils: code=%#v permissions=%v", replay, replayPermissions)
	}

	denied := code
	denied.DeviceCodeHash = "device-hash-denied"
	denied.UserCodeHash = "user-hash-denied"
	if err := db.OAuth.CreateDeviceCode(ctx, denied, permissions[:1]); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.OAuth.DenyDeviceCode(ctx, denied.UserCodeHash, 1500); err != nil || !ok {
		t.Fatalf("DenyDeviceCode should deny pending code: ok=%v err=%v", ok, err)
	}
	if ok, err := db.OAuth.DenyDeviceCode(ctx, denied.UserCodeHash, 1510); err != nil || ok {
		t.Fatalf("DenyDeviceCode should reject non-pending code: ok=%v err=%v", ok, err)
	}
	got, gotPermissions, err = db.OAuth.GetDeviceCodeByUserCodeHash(ctx, denied.UserCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	deniedAt := int64(1500)
	wantDenied := denied
	wantDenied.Status = "denied"
	wantDenied.DeniedAt = &deniedAt
	if !reflect.DeepEqual(got, &wantDenied) {
		t.Fatalf("denied device code mismatch:\n got=%#v\nwant=%#v", got, &wantDenied)
	}
	if !reflect.DeepEqual(gotPermissions, permissions[:1]) {
		t.Fatalf("denied permissions=%v want=%v", gotPermissions, permissions[:1])
	}
	missingCode, missingPermissions, err := db.OAuth.GetDeviceCodeByUserCodeHash(ctx, "missing-user-code")
	if err != nil {
		t.Fatal(err)
	}
	if missingCode != nil || missingPermissions != nil {
		t.Fatalf("missing device code should return nils: code=%#v permissions=%v", missingCode, missingPermissions)
	}
	missingCode, missingPermissions, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, "missing-device-code")
	if err != nil {
		t.Fatal(err)
	}
	if missingCode != nil || missingPermissions != nil {
		t.Fatalf("missing device hash should return nils: code=%#v permissions=%v", missingCode, missingPermissions)
	}
	if err := db.OAuth.MarkDeviceCodePolled(ctx, "missing-device-code", 1600); err != nil {
		t.Fatal(err)
	}
	expired := code
	expired.DeviceCodeHash = "device-hash-expired"
	expired.UserCodeHash = "user-hash-expired"
	expired.ExpiresAt = 1000
	if err := db.OAuth.CreateDeviceCode(ctx, expired, permissions[:1]); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.OAuth.ApproveDeviceCode(ctx, expired.UserCodeHash, user.ID, subjectID, 1200); err != nil || ok {
		t.Fatalf("ApproveDeviceCode should reject expired pending code: ok=%v err=%v", ok, err)
	}
	if ok, err := db.OAuth.DenyDeviceCode(ctx, expired.UserCodeHash, 1200); err != nil || ok {
		t.Fatalf("DenyDeviceCode should reject expired pending code: ok=%v err=%v", ok, err)
	}
	got, gotPermissions, err = db.OAuth.GetDeviceCodeByDeviceCodeHash(ctx, expired.DeviceCodeHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &expired) || !reflect.DeepEqual(gotPermissions, permissions[:1]) {
		t.Fatalf("expired device code should remain pending: got=%#v perms=%v", got, gotPermissions)
	}
}
