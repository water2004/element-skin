package service

import (
	"image"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/service/microsoft"
	"element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/service/yggdrasil"
)

type Site = sitepkg.Site
type Yggdrasil = yggdrasil.Yggdrasil
type Settings = settings.Settings
type Fallback = fallback.Fallback
type FallbackResponse = fallback.FallbackResponse
type ImportService = imports.ImportService
type TextureAsset = imports.TextureAsset
type MicrosoftAuthFlow = microsoft.MicrosoftAuthFlow
type MicrosoftAuthClient = microsoft.MicrosoftAuthClient
type MicrosoftHTTPClient = microsoft.MicrosoftHTTPClient
type TextureStorage = texture.TextureStorage

var SettingDefaults = settings.SettingDefaults

func MicrosoftAuthorizationURL(clientID, redirectURI, state string) string {
	return microsoft.MicrosoftAuthorizationURL(clientID, redirectURI, state)
}

func ValidateFallbackEndpoints(value any) ([]database.FallbackEndpoint, error) {
	return settings.ValidateFallbackEndpoints(value)
}

func ValidateFallbackServices(value any) ([]map[string]any, error) {
	return settings.ValidateFallbackServices(value)
}

func NewTextureStorage(dir string) (*texture.TextureStorage, error) {
	return texture.NewTextureStorage(dir)
}

func TexturePixelHash(img image.Image) string {
	return texture.TexturePixelHash(img)
}
