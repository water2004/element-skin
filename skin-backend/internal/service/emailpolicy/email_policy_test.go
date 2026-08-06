package emailpolicy_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	emailpolicysvc "element-skin/backend/internal/service/emailpolicy"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestNormalizeCanonicalizesSortsAndRejectsInvalidInputsExactly(t *testing.T) {
	got, err := emailpolicysvc.Normalize(model.EmailSuffixPolicy{
		Mode:      model.EmailSuffixModeAllowlist,
		Allowlist: []string{" QQ.COM ", "@Example.com"},
		Denylist:  []string{"Blocked.Test"},
	})
	want := model.EmailSuffixPolicy{
		Mode:      model.EmailSuffixModeAllowlist,
		Allowlist: []string{"@example.com", "@qq.com"},
		Denylist:  []string{"@blocked.test"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized=%#v err=%v want=%#v", got, err, want)
	}

	cases := []struct {
		name   string
		input  model.EmailSuffixPolicy
		detail string
	}{
		{"mode", model.EmailSuffixPolicy{Mode: "other"}, "invalid email suffix policy mode"},
		{"empty allowlist", model.EmailSuffixPolicy{Mode: model.EmailSuffixModeAllowlist}, "email suffix allowlist cannot be empty while enabled"},
		{"invalid suffix", model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDisabled, Allowlist: []string{"not-a-domain"}}, `invalid email suffix "not-a-domain"`},
		{"duplicate", model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDisabled, Denylist: []string{"QQ.com", "@qq.com"}}, `duplicate email suffix "@qq.com" in denylist`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := emailpolicysvc.Normalize(tc.input)
			if !reflect.DeepEqual(got, model.EmailSuffixPolicy{}) || !httpErrorIs(err, http.StatusBadRequest, tc.detail) {
				t.Fatalf("Normalize()=%#v err=%#v want empty exact HTTP 400 %q", got, err, tc.detail)
			}
		})
	}

	tooMany := make([]string, emailpolicysvc.MaxSuffixesPerList+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("suffix-%d.test", index)
	}
	if _, err := emailpolicysvc.Normalize(model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDisabled, Allowlist: tooMany}); !httpErrorIs(err, http.StatusBadRequest, "email suffix allowlist cannot contain more than 256 entries") {
		t.Fatalf("oversized allowlist error=%#v", err)
	}
}

func TestServiceEnforcesLiteralCaseInsensitiveSuffixModes(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := emailpolicysvc.Service{DB: db}

	allow := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeAllowlist, Allowlist: []string{"@example.com"}, Denylist: []string{}}
	if err := db.EmailPolicies.Replace(t.Context(), allow); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequireAllowed(t.Context(), " User@Example.COM "); err != nil {
		t.Fatalf("exact suffix should be allowed: %v", err)
	}
	if err := svc.RequireAllowed(t.Context(), "user@sub.example.com"); !httpErrorIs(err, http.StatusBadRequest, "Email suffix is not allowed") {
		t.Fatalf("subdomain suffix should not inherit allowlist: %#v", err)
	}

	deny := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDenylist, Allowlist: []string{}, Denylist: []string{"@blocked.test"}}
	if err := db.EmailPolicies.Replace(t.Context(), deny); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequireAllowed(t.Context(), "user@allowed.test"); err != nil {
		t.Fatalf("unmatched denylist suffix should be allowed: %v", err)
	}
	if err := svc.RequireAllowed(t.Context(), "user@BLOCKED.TEST"); !httpErrorIs(err, http.StatusBadRequest, "Email suffix is not allowed") {
		t.Fatalf("matched denylist suffix error=%#v", err)
	}
}

func TestServiceUpdateRequiresSettingsPermissionPersistsAndInvalidatesPublicCache(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cache := testutil.NewMemoryRedis()
	svc := emailpolicysvc.Service{DB: db, Redis: cache}
	if err := cache.SetPublicSettings(t.Context(), map[string]any{"site_name": "stale"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	input := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDenylist, Allowlist: []string{"Allowed.Test"}, Denylist: []string{"Blocked.Test"}}
	if err := svc.Update(t.Context(), permission.Actor{}, input); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("missing update permission error=%#v", err)
	}
	if err := svc.Update(t.Context(), actor("site_settings.update.any"), input); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetPublicSettings(t.Context()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("public settings cache should be invalidated, got %v", err)
	}
	got, err := svc.Read(t.Context(), actor("site_settings.read.any"))
	want := model.EmailSuffixPolicy{Mode: model.EmailSuffixModeDenylist, Allowlist: []string{"@allowed.test"}, Denylist: []string{"@blocked.test"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("read policy=%#v err=%v want=%#v", got, err, want)
	}
	if _, err := svc.Read(t.Context(), permission.Actor{}); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("missing read permission error=%#v", err)
	}
}

func actor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{Permissions: bits}
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}
