package permission_test

import (
	"testing"

	"element-skin/backend/internal/permission"
)

func TestComposeIDEncodesResourceActionAndScopeExactly(t *testing.T) {
	id, err := permission.ComposeID(0x12, 0x34, 0x56)
	if err != nil {
		t.Fatalf("ComposeID returned error: %v", err)
	}
	if uint64(id) != 0x0000_0012_0034_0056 {
		t.Fatalf("id mismatch: %#x", uint64(id))
	}
	if id.ResourceID() != 0x12 || id.ActionID() != 0x34 || id.ScopeID() != 0x56 || !id.Valid() {
		t.Fatalf("decoded id mismatch: resource=%#x action=%#x scope=%#x valid=%v", id.ResourceID(), id.ActionID(), id.ScopeID(), id.Valid())
	}
}

func TestComposeIDRejectsZeroPartsExactly(t *testing.T) {
	cases := []struct {
		name     string
		resource permission.ResourceID
		action   permission.ActionID
		scope    permission.ScopeID
	}{
		{name: "resource", resource: 0, action: 1, scope: 1},
		{name: "action", resource: 1, action: 0, scope: 1},
		{name: "scope", resource: 1, action: 1, scope: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if id, err := permission.ComposeID(tc.resource, tc.action, tc.scope); err == nil || id != 0 {
				t.Fatalf("ComposeID should reject zero %s: id=%#x err=%v", tc.name, uint64(id), err)
			}
		})
	}
}

func TestIDRejectsNonZeroReservedCategorySegmentExactly(t *testing.T) {
	id := permission.ID(0x0001_0001_0001_0001)
	if id.Valid() {
		t.Fatalf("id with reserved category segment should be invalid: %#x", uint64(id))
	}
	if id.ResourceID() != 1 || id.ActionID() != 1 || id.ScopeID() != 1 {
		t.Fatalf("low segments should still decode exactly: resource=%d action=%d scope=%d", id.ResourceID(), id.ActionID(), id.ScopeID())
	}
}

func TestMustComposeIDPanicsOnZeroPart(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustComposeID should panic on zero resource")
		}
	}()
	permission.MustComposeID(0, 1, 1)
}
