package fallback

import (
	"net/http"

	"element-skin/backend/internal/database"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Fallback struct {
	DB       *database.DB
	Client   *http.Client
	Settings settingssvc.Settings
}

func (f Fallback) settings() settingssvc.Settings {
	if f.Settings.DB == nil {
		f.Settings.DB = f.DB
	}
	return f.Settings
}

type FallbackResponse struct {
	Status int
	Body   []byte
}
