package service_test

import (
	"image"
	"testing"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/service/microsoft"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/texture"
)

func TestServiceFacadeExportsDomainServicesExactly(t *testing.T) {
	if service.SettingDefaults["site_name"] != settings.SettingDefaults["site_name"] {
		t.Fatalf("settings defaults facade mismatch: got %q want %q", service.SettingDefaults["site_name"], settings.SettingDefaults["site_name"])
	}
	if got := service.MicrosoftAuthorizationURL("client", "https://redirect", "state"); got != microsoft.MicrosoftAuthorizationURL("client", "https://redirect", "state") {
		t.Fatalf("microsoft auth URL facade mismatch: %q", got)
	}
	storage, err := service.NewTextureStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := any(storage).(*texture.TextureStorage); !ok {
		t.Fatalf("texture storage facade returned wrong type: %T", storage)
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if service.TexturePixelHash(img) != texture.TexturePixelHash(img) {
		t.Fatal("texture pixel hash facade should delegate exactly")
	}
}
