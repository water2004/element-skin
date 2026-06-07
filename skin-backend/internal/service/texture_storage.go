package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const MaxTextureDimension = 1024

type TextureStorage struct {
	Dir string
}

func NewTextureStorage(dir string) (*TextureStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &TextureStorage{Dir: dir}, nil
}

func (s *TextureStorage) ProcessAndSave(data []byte, textureType string) (string, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("Image must be PNG format")
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if !validTextureDimensions(w, h, strings.EqualFold(textureType, "cape")) {
		return "", fmt.Errorf("invalid texture dimensions")
	}
	hash := TexturePixelHash(img)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(s.Dir, hash+".png"), out.Bytes(), 0o644); err != nil {
		return "", err
	}
	return hash, nil
}

func (s *TextureStorage) DeleteFile(hash string) error {
	err := os.Remove(filepath.Join(s.Dir, hash+".png"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func validTextureDimensions(w, h int, cape bool) bool {
	if w <= 0 || h <= 0 || w > MaxTextureDimension || h > MaxTextureDimension {
		return false
	}
	if cape {
		return (w%64 == 0 && h%32 == 0) || (w%22 == 0 && h%17 == 0)
	}
	return (w%64 == 0 && h == w) || (w%64 == 0 && h*2 == w)
}

func TexturePixelHash(img image.Image) string {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	buf := make([]byte, 8+w*h*4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(w))
	binary.BigEndian.PutUint32(buf[4:8], uint32(h))
	pos := 8
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			rgba := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if rgba.A == 0 {
				rgba.R, rgba.G, rgba.B = 0, 0, 0
			}
			buf[pos] = rgba.A
			buf[pos+1] = rgba.R
			buf[pos+2] = rgba.G
			buf[pos+3] = rgba.B
			pos += 4
		}
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
