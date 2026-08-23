package emailpolicy_test

import (
	"reflect"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestStoreReturnsDefaultAndReplacesBothListsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	got, err := db.EmailPolicies.Get(t.Context())
	wantDefault := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDisabled, Allowlist: []string{}, Denylist: []string{}}
	if err != nil || !reflect.DeepEqual(got, wantDefault) {
		t.Fatalf("default policy=%#v err=%v want=%#v", got, err, wantDefault)
	}

	want := model.EmailSuffixPolicy{
		Mode:      model.EmailSuffixModeAllowlist,
		Allowlist: []string{"@example.com", "@qq.com"},
		Denylist:  []string{"@blocked.example", "@invalid.test"},
	}
	if err := db.EmailPolicies.Replace(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got, err = db.EmailPolicies.Get(t.Context())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("replaced policy=%#v err=%v want=%#v", got, err, want)
	}
}

func TestStoreReplaceRollsBackModeAndRulesOnDuplicate(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	initial := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDenylist, Allowlist: []string{"@allowed.test"}, Denylist: []string{"@blocked.test"}}
	if err := db.EmailPolicies.Replace(t.Context(), initial); err != nil {
		t.Fatal(err)
	}
	invalid := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeAllowlist, Allowlist: []string{"@duplicate.test", "@duplicate.test"}, Denylist: []string{}}
	if err := db.EmailPolicies.Replace(t.Context(), invalid); err == nil {
		t.Fatal("duplicate suffix replacement should fail")
	}
	got, err := db.EmailPolicies.Get(t.Context())
	if err != nil || !reflect.DeepEqual(got, initial) {
		t.Fatalf("failed replacement changed policy: got=%#v err=%v want=%#v", got, err, initial)
	}
}
