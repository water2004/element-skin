package account_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
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
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}

func createAccountOAuthClient(t testing.TB, db *database.DB, ownerUserID, clientID string, codes ...string) model.OAuthClient {
	t.Helper()
	client := model.OAuthClient{
		ID:          clientID,
		OwnerUserID: ownerUserID,
		Name:        clientID,
		Description: "account service oauth fixture",
		RedirectURI: "https://" + clientID + ".example/callback",
		WebsiteURL:  "https://" + clientID + ".example",
		ClientType:  "confidential",
		SecretHash:  clientID + "-secret-hash",
		Status:      "active",
		CreatedAt:   2000,
		UpdatedAt:   2000,
	}
	if err := db.OAuth.CreateClient(context.Background(), client, accountOAuthPermissionIDs(codes...), nil); err != nil {
		t.Fatal(err)
	}
	return client
}

func accountOAuthPermissionIDs(codes ...string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	return ids
}

func assertAccountRowCount(t testing.TB, db *database.DB, query string, arg any, want int) {
	t.Helper()
	var got int
	if err := db.Pool.QueryRow(context.Background(), query, arg).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("row count mismatch for %q arg=%v: got=%d want=%d", query, arg, got, want)
	}
}

func accountOAuthGrantByID(grants []model.OAuthGrant, id string) *model.OAuthGrant {
	for i := range grants {
		if grants[i].ID == id {
			return &grants[i]
		}
	}
	return nil
}

func accountNoticeByTitle(t testing.TB, notices []model.NoticeView, title string) model.NoticeView {
	t.Helper()
	for _, notice := range notices {
		if notice.Title == title {
			return notice
		}
	}
	t.Fatalf("missing notice title %q in %#v", title, notices)
	return model.NoticeView{}
}

type accountFailStore struct {
	redisstore.Store
	failInvalidate bool
	failYggDelete  bool
}

func (s *accountFailStore) InvalidateAuthUser(ctx context.Context, userID string) error {
	if s.failInvalidate {
		return errors.New("auth cache invalidation failed")
	}
	return s.Store.InvalidateAuthUser(ctx, userID)
}

func (s *accountFailStore) DeleteYggTokensByUser(ctx context.Context, userID string) error {
	if s.failYggDelete {
		return errors.New("ygg token deletion failed")
	}
	return s.Store.DeleteYggTokensByUser(ctx, userID)
}

func assertAccountPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func stringSliceSetEquals(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, item := range got {
		seen[item]++
	}
	for _, item := range want {
		seen[item]--
		if seen[item] < 0 {
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
