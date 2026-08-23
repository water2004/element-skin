package shared

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/config"
)

func SetWebSessionCookies(w http.ResponseWriter, cfg config.Config, access, refresh string, refreshMaxAgeSeconds int) {
	http.SetCookie(w, WebSessionCookie(cfg, "access_token", access, cfg.AccessMinutes*60))
	http.SetCookie(w, WebSessionCookie(cfg, "refresh_token", refresh, refreshMaxAgeSeconds))
}

func ClearWebSessionCookies(w http.ResponseWriter, cfg config.Config) {
	http.SetCookie(w, WebSessionCookie(cfg, "access_token", "", -1))
	http.SetCookie(w, WebSessionCookie(cfg, "refresh_token", "", -1))
}

func WebSessionCookie(cfg config.Config, name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(strings.ToLower(cfg.SiteURL), "https://"),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}
