package homepage

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) UploadImage(ctx context.Context, actor permission.Actor, upload UploadInput) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaCreatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	if upload.Filename == "" {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "required"}
	}
	if int64(len(upload.Data)) > MaxImageBytes {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "too_large"}
	}
	ext := strings.ToLower(filepath.Ext(upload.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "unsupported"}
	}
	if err := validateImageData(upload.Data, ext); err != nil {
		if errors.Is(err, errImageDimensions) {
			return model.HomepageMedia{}, util.HTTPError{
				Status: http.StatusBadRequest, Object: "homepage_image", Operation: "validate", Reason: "too_large",
			}
		}
		return model.HomepageMedia{}, util.HTTPError{
			Status: http.StatusBadRequest, Object: "homepage_image", Operation: "decode", Reason: "invalid",
		}
	}
	values, err := ParseMediaValues(upload.Fields, "image")
	if err != nil {
		return model.HomepageMedia{}, err
	}
	return s.createImage(ctx, upload.Filename, ext, upload.Data, values)
}

func (s Service) UploadPanorama(ctx context.Context, actor permission.Actor, upload UploadInput) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaCreatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	if upload.Filename == "" {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "required"}
	}
	if int64(len(upload.Data)) > MaxPanoramaBytes {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "too_large"}
	}
	if strings.ToLower(filepath.Ext(upload.Filename)) != ".zip" {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "upload_file", Operation: "validate", Reason: "unsupported"}
	}
	faces, err := ReadPanoramaZip(upload.Data)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	values, err := ParseMediaValues(upload.Fields, "panorama")
	if err != nil {
		return model.HomepageMedia{}, err
	}
	return s.createPanorama(ctx, upload.Filename, faces, values)
}

func (s Service) createImage(ctx context.Context, filename, ext string, data []byte, values MediaValues) (model.HomepageMedia, error) {
	item, path, err := s.newMedia(ctx, "image", filename, ext, values)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := s.DB.HomepageMedia.Create(ctx, item); err != nil {
		_ = os.Remove(path)
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		_, _ = s.DB.HomepageMedia.Delete(ctx, item.ID)
		_ = os.Remove(path)
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) createPanorama(ctx context.Context, filename string, faces map[string][]byte, values MediaValues) (model.HomepageMedia, error) {
	item, dir, err := s.newMedia(ctx, "panorama", filename, "", values)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.HomepageMedia{}, err
	}
	for name, content := range faces {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return model.HomepageMedia{}, err
		}
	}
	if err := s.DB.HomepageMedia.Create(ctx, item); err != nil {
		_ = os.RemoveAll(dir)
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		_, _ = s.DB.HomepageMedia.Delete(ctx, item.ID)
		_ = os.RemoveAll(dir)
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) newMedia(ctx context.Context, typ, title, ext string, values MediaValues) (model.HomepageMedia, string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	order, err := s.DB.HomepageMedia.NextSortOrder(ctx)
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	duration := values.DurationMS
	if duration == 0 {
		duration = 6000
	}
	if typ == "panorama" && values.DurationMS == 0 {
		duration = 9000
	}
	if duration < 1000 || duration > 60000 {
		return model.HomepageMedia{}, "", util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_duration", Operation: "validate", Reason: "out_of_range"}
	}
	now := database.NowMS()
	storagePath := id + ext
	path := filepath.Join(s.CarouselDir, storagePath)
	if typ == "panorama" {
		storagePath = id
		path = filepath.Join(s.CarouselDir, id)
	}
	if title == "" {
		title = typ
	}
	return model.HomepageMedia{
		ID:                  id,
		Type:                typ,
		Title:               title,
		StoragePath:         filepath.ToSlash(storagePath),
		OverlayOpacityLight: values.OverlayOpacityLight,
		OverlayOpacityDark:  values.OverlayOpacityDark,
		StartYaw:            values.StartYaw,
		StartPitch:          values.StartPitch,
		YawSpeedDPS:         values.YawSpeedDPS,
		PitchSpeedDPS:       values.PitchSpeedDPS,
		SortOrder:           order,
		Enabled:             true,
		DurationMS:          duration,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, path, nil
}
