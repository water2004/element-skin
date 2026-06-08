package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Handler struct {
	cfg config.Config
	db  *database.DB
	ygg yggpkg.Yggdrasil
}

func New(cfg config.Config, db *database.DB, ygg yggpkg.Yggdrasil) Handler {
	return Handler{cfg: cfg, db: db, ygg: ygg}
}
