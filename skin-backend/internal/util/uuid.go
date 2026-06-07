package util

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

func RandomUUIDNoDash() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}

func OfflineUUIDNoDash(name string) string {
	h := md5.Sum([]byte("OfflinePlayer:" + name))
	b := h[:]
	b[6] = (b[6] & 0x0f) | 0x30
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

func StripUUIDDashes(id string) string {
	return strings.ReplaceAll(id, "-", "")
}
