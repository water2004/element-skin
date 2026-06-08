package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	settingssvc "element-skin/backend/internal/service/settings"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	settings settingssvc.Settings
	ygg      yggpkg.Yggdrasil
}

func New(cfg config.Config, db *database.DB, settings settingssvc.Settings, ygg yggpkg.Yggdrasil) Handler {
	if settings.DB == nil {
		settings.DB = db
	}
	if ygg.Settings.DB == nil {
		ygg.Settings = settings
	}
	return Handler{cfg: cfg, db: db, settings: settings, ygg: ygg}
}
