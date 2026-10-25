package mail

import (
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
)

func TestSMTPSSLOnlyEnablesForCanonicalTrue(t *testing.T) {
	cache := redisstore.NewMemoryStore()
	for key, value := range map[string]string{
		"smtp_host": "smtp.example", "smtp_port": "465", "smtp_user": "sender@example.com",
		"smtp_password": "fixture-password", "smtp_sender": "sender@example.com",
	} {
		if err := cache.SetSetting(t.Context(), key, value, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	sender := SMTP{Settings: settingssvc.Settings{Redis: cache}}
	for _, fixture := range []struct {
		raw  string
		want bool
	}{{"true", true}, {"false", false}, {"1", false}, {"0", false}, {"TRUE", false}, {" true ", false}} {
		t.Run(fixture.raw, func(t *testing.T) {
			if err := cache.SetSetting(t.Context(), "smtp_ssl", fixture.raw, time.Minute); err != nil {
				t.Fatal(err)
			}
			cfg, err := sender.config(t.Context())
			if err != nil || cfg.implicitTLS != fixture.want {
				t.Fatalf("SMTP SSL(%q)=%t err=%v; want %t", fixture.raw, cfg.implicitTLS, err, fixture.want)
			}
		})
	}
}
