package permissions_test

import (
	"errors"
	"sort"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/util"
)

func actorWithPermissions(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   permissiondb.SubjectIDForUser(userID),
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}

func containsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringSliceEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func int64SliceEqual(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func hasOverride(overrides []permissionssvc.PermissionOverrideResponse, code, effect string) bool {
	for _, override := range overrides {
		if override.PermissionCode == code && override.Effect == effect && override.CreatedAt > 0 {
			return true
		}
	}
	return false
}

func oauthGrantByID(grants []model.OAuthGrant, id string) *model.OAuthGrant {
	for i := range grants {
		if grants[i].ID == id {
			return &grants[i]
		}
	}
	return nil
}

func permissionNoticeByTitle(t testing.TB, notices []model.NoticeView, title string) model.NoticeView {
	t.Helper()
	for _, notice := range notices {
		if notice.Title == title {
			return notice
		}
	}
	t.Fatalf("missing notice title %q in %#v", title, notices)
	return model.NoticeView{}
}

func permissionTestIDs(codes ...string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
