package notice_test

import (
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func httpError(err error, status int, detail string) bool {
	he, ok := err.(util.HTTPError)
	return ok && he.Status == status && he.Error() == detail
}

func ptrInt64(v int64) *int64 { return &v }

func ptrBool(v bool) *bool { return &v }

func ptrString(v string) *string { return &v }

func noticeActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func noticeManagerActor(userID string) permission.Actor {
	return noticeActor(
		userID,
		"notice.read.any",
		"notice.create.any",
		"notice.update.any",
		"notice.delete.any",
	)
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
