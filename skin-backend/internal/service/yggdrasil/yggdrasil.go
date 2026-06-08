package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

type Yggdrasil struct {
	DB       *database.DB
	Cfg      config.Config
	Redis    redisstore.Store
	Signer   *Signer
	Settings settingssvc.Settings
}

func New(db *database.DB, cfg config.Config, redis redisstore.Store, settings settingssvc.Settings) (Yggdrasil, error) {
	signer, err := NewSigner(cfg)
	if err != nil {
		return Yggdrasil{}, err
	}
	return Yggdrasil{DB: db, Cfg: cfg, Redis: redis, Signer: signer, Settings: settings}, nil
}

func yggErr(status int, code, msg string) error {
	return util.HTTPError{Status: status, Detail: msg, YggError: code}
}

func (y Yggdrasil) signer() (*Signer, error) {
	if y.Signer != nil {
		return y.Signer, nil
	}
	return NewSigner(y.Cfg)
}

func (y Yggdrasil) settings() settingssvc.Settings {
	return y.Settings
}

func (y Yggdrasil) publicTextureBaseURL() string {
	if y.Cfg.APIURL != "" {
		return y.Cfg.APIURL
	}
	return y.Cfg.SiteURL
}
