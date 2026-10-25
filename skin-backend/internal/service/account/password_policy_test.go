package account_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

var passwordSettingFailure = errors.New("password setting unavailable")

type passwordSettingFailureStore struct {
	redisstore.Store
	failWrite bool
}

func (s passwordSettingFailureStore) GetSetting(ctx context.Context, key string) (string, error) {
	if key == "enable_strong_password_check" && !s.failWrite {
		return "", passwordSettingFailure
	}
	return s.Store.GetSetting(ctx, key)
}

func (s passwordSettingFailureStore) SetSetting(ctx context.Context, key, value string, ttl time.Duration) error {
	if key == "enable_strong_password_check" && s.failWrite {
		return passwordSettingFailure
	}
	return s.Store.SetSetting(ctx, key, value, ttl)
}

func TestPasswordPolicyFailuresPreserveHashRefreshYggAndAuthCache(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	admin := testutil.CreateUser(t, db, "boundary-admin@example.com", "Password123", "BoundaryAdmin", true)
	for _, failure := range []string{"weak", "too_long", "setting_read", "setting_write"} {
		for _, operation := range []string{"self", "admin"} {
			t.Run(failure+"_"+operation, func(t *testing.T) {
				user := testutil.CreateUser(t, db, failure+"-"+operation+"@example.com", "Password123", failure+operation, false)
				before, err := db.Users.GetByID(ctx, user.ID)
				if err != nil {
					t.Fatal(err)
				}
				cache := redisstore.NewMemoryStore()
				if err := db.Settings.Set(ctx, "enable_strong_password_check", "true"); err != nil {
					t.Fatal(err)
				}
				cached := redisstore.AuthUser{ID: user.ID}
				ygg := model.Token{AccessToken: "boundary-" + user.ID, UserID: user.ID, CreatedAt: database.NowMS()}
				if err := cache.SetAuthUser(ctx, cached, time.Hour); err != nil {
					t.Fatal(err)
				}
				if err := cache.SetYggToken(ctx, ygg, time.Hour); err != nil {
					t.Fatal(err)
				}
				if err := db.Tokens.AddRefresh(ctx, user.ID, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
					t.Fatal(err)
				}
				refreshBefore, err := db.Tokens.GetRefresh(ctx, user.ID)
				if err != nil {
					t.Fatal(err)
				}
				var store redisstore.Store = cache
				passwords := make(map[string]testutil.PasswordPolicyCase)
				for _, fixture := range testutil.PasswordPolicyCases(t) {
					passwords[fixture.Name] = fixture
				}
				fixture := passwords["eight digits"]
				switch failure {
				case "too_long":
					fixture = passwords["73 UTF8 bytes"]
				case "setting_read", "setting_write":
					fixture = passwords["digits and punctuation"]
					store = passwordSettingFailureStore{Store: cache, failWrite: failure == "setting_write"}
				}
				svc := accountsvc.AccountService{DB: db, Redis: store}
				if operation == "self" {
					err = svc.ChangePasswordSelf(ctx, accountActor(t, db, user.ID), "Password123", fixture.Password)
				} else {
					err = svc.ResetPassword(ctx, actorWithPermissions(admin.ID, "account.update.any"), accountsvc.ResetPasswordInput{UserID: user.ID, NewPassword: fixture.Password})
				}
				switch failure {
				case "weak", "too_long":
					var apiErr util.HTTPError
					if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Error() != "password.validate.invalid" || !reflect.DeepEqual(apiErr.Params, map[string]any{"rules": fixture.StrongErrors}) {
						t.Fatalf("error=%#v; want policy HTTP400 with rules=%#v", err, fixture.StrongErrors)
					}
				default:
					if !errors.Is(err, passwordSettingFailure) {
						t.Fatalf("error=%v; want exact setting failure", err)
					}
				}
				after, err := db.Users.GetByID(ctx, user.ID)
				if err != nil || !reflect.DeepEqual(after, before) {
					t.Fatalf("failure changed user: err=%v", err)
				}
				refreshAfter, err := db.Tokens.GetRefresh(ctx, user.ID)
				if err != nil || !reflect.DeepEqual(refreshAfter, refreshBefore) {
					t.Fatalf("failure changed refresh: before=%#v after=%#v err=%v", refreshBefore, refreshAfter, err)
				}
				actualCache, err := cache.GetAuthUser(ctx, user.ID)
				if err != nil || !reflect.DeepEqual(actualCache, cached) {
					t.Fatalf("failure changed auth cache: got=%#v err=%v", actualCache, err)
				}
				actualYgg, err := cache.GetYggToken(ctx, ygg.AccessToken)
				if err != nil || !reflect.DeepEqual(actualYgg, ygg) {
					t.Fatalf("failure changed Ygg token: got=%#v err=%v", actualYgg, err)
				}
			})
		}
	}
}
