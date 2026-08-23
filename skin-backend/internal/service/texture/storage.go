package texture

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const MaxTextureDimension = 1024

var (
	ErrInvalidPNG               = errors.New("invalid PNG")
	ErrInvalidTextureDimensions = errors.New("invalid texture dimensions")
)

type TextureStorage struct {
	Dir string
}

func NewTextureStorage(dir string) (*TextureStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &TextureStorage{Dir: dir}, nil
}

func (s *TextureStorage) ProcessAndSaveTracked(data []byte, textureType string) (string, bool, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return "", false, ErrInvalidPNG
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if !validTextureDimensions(w, h, strings.EqualFold(textureType, "cape")) {
		return "", false, ErrInvalidTextureDimensions
	}
	hash := TexturePixelHash(img)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return "", false, err
	}
	path := filepath.Join(s.Dir, hash+".png")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return hash, false, nil
	}
	if err != nil {
		return "", false, err
	}
	if _, err := file.Write(out.Bytes()); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", false, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", false, err
	}
	return hash, true, nil
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
