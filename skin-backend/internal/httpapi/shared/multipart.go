package shared

import (
	"io"
	"net/http"

	"element-skin/backend/internal/util"
)

const (
	MaxMultipartParts      = 32
	MaxMultipartFieldBytes = 4096
)

type MultipartUpload struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func ReadMultipartUpload(req *http.Request, fileField string, maxBytes int64) (MultipartUpload, error) {
	reader, err := req.MultipartReader()
	if err != nil {
		return MultipartUpload{}, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"}
	}
	out := MultipartUpload{Fields: map[string]string{}}
	partCount := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return MultipartUpload{}, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"}
		}
		partCount++
		if partCount > MaxMultipartParts {
			_ = part.Close()
			return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_field", Operation: "decode", Reason: "exceeded"}
		}
		formName := part.FormName()
		if formName == "" {
			_ = part.Close()
			continue
		}
		if formName == fileField {
			if out.Filename != "" {
				_ = part.Close()
				return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_field", Operation: "decode", Reason: "conflict"}
			}
			out.Filename = part.FileName()
			data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
			_ = part.Close()
			if err != nil {
				return MultipartUpload{}, err
			}
			if int64(len(data)) > maxBytes {
				return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_file", Operation: "validate", Reason: "too_large"}
			}
			out.Data = data
			continue
		}
		if _, exists := out.Fields[formName]; exists {
			_ = part.Close()
			return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_field", Operation: "decode", Reason: "conflict"}
		}
		data, err := io.ReadAll(io.LimitReader(part, MaxMultipartFieldBytes+1))
		_ = part.Close()
		if err != nil {
			return MultipartUpload{}, err
		}
		if len(data) > MaxMultipartFieldBytes {
			return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_field", Operation: "decode", Reason: "too_large"}
		}
		out.Fields[formName] = string(data)
	}
	if out.Filename == "" {
		return MultipartUpload{}, util.HTTPError{Status: 400, Object: "upload_file", Operation: "validate", Reason: "required"}
	}
	return out, nil
}
