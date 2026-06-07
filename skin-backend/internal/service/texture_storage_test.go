package service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTextureStorageCreatesDirectoryAndSaves(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "textures")
	storage, err := NewTextureStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("texture directory should exist: %v", err)
	}
	hash, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{255, 0, 0, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 64 {
		t.Fatalf("expected sha256 hex hash, got %q", hash)
	}
	if _, err := os.Stat(filepath.Join(dir, hash+".png")); err != nil {
		t.Fatalf("saved png missing: %v", err)
	}
	if err := storage.DeleteFile(hash); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, hash+".png")); !os.IsNotExist(err) {
		t.Fatal("texture file should be deleted")
	}
	if err := storage.DeleteFile(hash); err != nil {
		t.Fatal("delete should be idempotent")
	}
}

func TestTextureStorageHashStabilityAndAlphaZero(t *testing.T) {
	storage, err := NewTextureStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h1, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("same pixels should hash identically")
	}
	h3, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{200, 100, 50, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 {
		t.Fatal("different pixels should produce different hash")
	}
	transparentA, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 0}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	transparentB, err := storage.ProcessAndSave(pngBytes(t, 64, 64, color.RGBA{200, 100, 50, 0}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if transparentA != transparentB {
		t.Fatal("transparent pixels must zero RGB before hashing")
	}
}

func TestTextureStorageValidCapeAndInvalidInputs(t *testing.T) {
	storage, err := NewTextureStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ProcessAndSave(pngBytes(t, 64, 32, color.RGBA{1, 2, 3, 255}), "cape"); err != nil {
		t.Fatalf("valid cape rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"bad skin 63", pngBytes(t, 63, 63, color.RGBA{1, 2, 3, 255})},
		{"bad skin 100", pngBytes(t, 100, 100, color.RGBA{1, 2, 3, 255})},
		{"oversize", pngBytes(t, 2048, 64, color.RGBA{1, 2, 3, 255})},
		{"not png", []byte("this is definitely not a png")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := storage.ProcessAndSave(tc.data, "skin"); err == nil {
				t.Fatal("expected invalid texture to be rejected")
			}
		})
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.ProcessAndSave(jpg.Bytes(), "skin"); err == nil || !strings.Contains(err.Error(), "PNG") {
		t.Fatalf("jpeg should be rejected as non-png: %v", err)
	}
}
