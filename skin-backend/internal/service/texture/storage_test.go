package texture

import (
	"bytes"
	"errors"
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
	hash, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{255, 0, 0, 255}), "skin")
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
	h1, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	h2, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("same pixels should hash identically")
	}
	h3, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{200, 100, 50, 255}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 {
		t.Fatal("different pixels should produce different hash")
	}
	transparentA, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 0}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	transparentB, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 64, color.RGBA{200, 100, 50, 0}), "skin")
	if err != nil {
		t.Fatal(err)
	}
	if transparentA != transparentB {
		t.Fatal("transparent pixels must zero RGB before hashing")
	}
}

func TestTexturePixelHashMatchesYggdrasilReferenceVector(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	pixels := [][]color.NRGBA{
		{{R: 255, A: 255}, {B: 255, A: 255}, {R: 255, B: 255, A: 255}},
		{{G: 255, A: 255}, {R: 24, G: 48, B: 96, A: 0}, {R: 255, G: 255, A: 255}},
	}
	for x := range pixels {
		for y := range pixels[x] {
			img.SetNRGBA(x, y, pixels[x][y])
		}
	}

	const expected = "47a4c518f80f94ad8737713e0325a98e1f2647f962b9a646f58cd0bbd5afe683"
	if got := TexturePixelHash(img); got != expected {
		t.Fatalf("TexturePixelHash(reference)=%q; want %q", got, expected)
	}
}

func TestTexturePixelHashDoesNotPremultiplySemiTransparentPixels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}

	const expected = "514253a25317e04c5ba9f8b3c468800f7e73ced124aef621f12cc6bbf0216a11"
	if got := TexturePixelHash(img); got != expected {
		t.Fatalf("TexturePixelHash(semitransparent)=%q; want v2/Yggdrasil hash %q", got, expected)
	}
}

func TestProcessAndSaveTrackedReportsOnlyTheCreatingWrite(t *testing.T) {
	storage, err := NewTextureStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := pngBytes(t, 64, 64, color.RGBA{90, 40, 180, 255})

	firstHash, firstCreated, err := storage.ProcessAndSaveTracked(data, "skin")
	if err != nil || firstHash == "" || !firstCreated {
		t.Fatalf("first tracked save should create the file: hash=%q created=%v err=%v", firstHash, firstCreated, err)
	}
	secondHash, secondCreated, err := storage.ProcessAndSaveTracked(data, "skin")
	if err != nil || secondHash != firstHash || secondCreated {
		t.Fatalf("second tracked save should reuse the content-addressed file: first=%q second=%q created=%v err=%v", firstHash, secondHash, secondCreated, err)
	}
}

func TestTextureStorageValidCapeAndInvalidInputs(t *testing.T) {
	storage, err := NewTextureStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.ProcessAndSaveTracked(pngBytes(t, 64, 32, color.RGBA{1, 2, 3, 255}), "cape"); err != nil {
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
			if _, _, err := storage.ProcessAndSaveTracked(tc.data, "skin"); err == nil {
				t.Fatal("expected invalid texture to be rejected")
			}
		})
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.ProcessAndSaveTracked(jpg.Bytes(), "skin"); err == nil || !strings.Contains(err.Error(), "PNG") {
		t.Fatalf("jpeg should be rejected as non-png: %v", err)
	}
}

func TestTextureStorageRejectsInvalidDirectoryWithoutLeavingFiles(t *testing.T) {
	root := t.TempDir()
	blockingFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	storage, err := NewTextureStorage(filepath.Join(blockingFile, "textures"))
	if storage != nil || err == nil {
		t.Fatalf("NewTextureStorage()=%#v, %v; want nil and directory creation error", storage, err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "mkdir" || filepath.Clean(pathErr.Path) != filepath.Clean(blockingFile) {
		t.Fatalf("directory error=%#v; want mkdir PathError for exact blocking path", err)
	}
	content, readErr := os.ReadFile(blockingFile)
	if readErr != nil || string(content) != "keep" {
		t.Fatalf("failed directory creation changed blocking file: content=%q err=%v", content, readErr)
	}
}

func TestTextureStorageWriteFailureReturnsNoHashOrCreatedFile(t *testing.T) {
	root := t.TempDir()
	blockingFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	storage := &TextureStorage{Dir: blockingFile}

	hash, created, err := storage.ProcessAndSaveTracked(
		pngBytes(t, 64, 64, color.RGBA{10, 20, 30, 255}),
		"skin",
	)
	if hash != "" || created || err == nil {
		t.Fatalf("write failure returned hash=%q created=%v err=%v; want empty, false, error", hash, created, err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "open" || filepath.Dir(pathErr.Path) != blockingFile {
		t.Fatalf("write error=%#v; want open PathError below blocking path", err)
	}
	content, readErr := os.ReadFile(blockingFile)
	if readErr != nil || string(content) != "keep" {
		t.Fatalf("failed texture write changed blocking file: content=%q err=%v", content, readErr)
	}
}
