package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func modelDeviceCode(t *testing.T, db *database.DB, clientID, deviceHash, rawUserCode string, codes []string, ttlOffset time.Duration) string {
	t.Helper()
	now := database.NowMS()
	userCode := strings.ToUpper(rawUserCode)
	record := model.OAuthDeviceCode{
		DeviceCodeHash: deviceHash,
		UserCodeHash:   util.HashRefreshToken(userCode),
		ClientID:       clientID,
		Status:         "pending",
		ExpiresAt:      now + int64(ttlOffset/time.Millisecond),
		CreatedAt:      now,
	}
	if err := db.OAuth.CreateDeviceCode(context.Background(), record, permissionIDsFromCodesForTest(codes)); err != nil {
		t.Fatal(err)
	}
	return userCode
}

func permissionIDsFromCodesForTest(codes []string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	return ids
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func withAuthReq(req oauth.AuthorizationRequest, mutate func(*oauth.AuthorizationRequest)) oauth.AuthorizationRequest {
	mutate(&req)
	return req
}

func newOAuthService(db *database.DB) oauth.Service {
	return oauth.Service{DB: db, Redis: redisstore.NewMemoryStore()}
}

func oauthNoticeReader(userID string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	bits.Set(permission.MustDefinitionByCode("notice.read.owned").BitIndex)
	return permission.Actor{SubjectID: "user:" + userID, UserID: userID, Permissions: bits}
}

func grantClientPermission(t *testing.T, db *database.DB, clientID, code string) {
	t.Helper()
	def := permission.MustDefinitionByCode(code)
	if err := db.Permissions.SetPermissionOverrideForSubject(context.Background(), permissiondb.SubjectIDForClient(clientID), def, "allow", ""); err != nil {
		t.Fatal(err)
	}
}

func activateOAuthClient(t *testing.T, db *database.DB, clientID string) {
	t.Helper()
	if ok, err := db.OAuth.UpdateClientStatus(context.Background(), clientID, oauth.StatusActive, database.NowMS()); err != nil || !ok {
		t.Fatalf("activate oauth client: ok=%v err=%v", ok, err)
	}
}

type oauthAccessDeleteFailStore struct {
	redisstore.Store
	err         error
	deletedHash string
}

type oauthClientAccessDeleteFailStore struct {
	redisstore.Store
	err error
}

type oauthClientAccessDeleteNthFailStore struct {
	redisstore.Store
	err        error
	calls      int
	failOnCall int
}

func (s *oauthClientAccessDeleteFailStore) DeleteOAuthAccessTokensByClient(_ context.Context, _ string) error {
	return s.err
}

func (s *oauthClientAccessDeleteNthFailStore) DeleteOAuthAccessTokensByClient(ctx context.Context, clientID string) error {
	s.calls++
	if s.calls == s.failOnCall {
		return s.err
	}
	return s.Store.DeleteOAuthAccessTokensByClient(ctx, clientID)
}

func (s *oauthAccessDeleteFailStore) DeleteOAuthAccessToken(_ context.Context, tokenHash string) error {
	s.deletedHash = tokenHash
	return s.err
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	if !isHTTPError(err, status, detail) {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}

func assertClosedPoolError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("closed database error mismatch: %v", err)
	}
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func isHTTPError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Error() == detail
}

func stringSetFromStrings(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
