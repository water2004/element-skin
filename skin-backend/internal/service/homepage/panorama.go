package homepage

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
)

func ReadPanoramaZip(data []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_archive", Operation: "decode", Reason: "invalid"}
	}
	required := map[string]bool{}
	for i := 0; i < 6; i++ {
		required["panorama_"+strconv.Itoa(i)+".png"] = false
	}
	out := map[string][]byte{}
	for _, f := range reader.File {
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "/") || strings.Contains(name, `\`) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_archive", Operation: "validate", Reason: "invalid"}
		}
		if _, ok := required[name]; !ok {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_archive", Operation: "validate", Reason: "invalid"}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxImageBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if len(content) > MaxImageBytes {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_face", Operation: "validate", Reason: "too_large"}
		}
		if err := validateImageData(content, ".png"); err != nil {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_face", Operation: "validate", Reason: "invalid"}
		}
		required[name] = true
		out[name] = content
	}
	for name, ok := range required {
		if !ok {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "panorama_face", Operation: "validate", Reason: "required", Params: map[string]any{"face": name}}
		}
	}
	return out, nil
}
