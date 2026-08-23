package homepage

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/database"
	dbhomepage "element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

const (
	MaxImageBytes     = 5 * 1024 * 1024
	MaxPanoramaBytes  = 50 * 1024 * 1024
	MaxImageDimension = 8192
	MaxImagePixels    = 32 * 1024 * 1024
)

var (
	homepageMediaReadPermission   = permission.MustDefinitionByCode("homepage_media.read.any")
	homepageMediaCreatePermission = permission.MustDefinitionByCode("homepage_media.create.any")
	homepageMediaUpdatePermission = permission.MustDefinitionByCode("homepage_media.update.any")
	homepageMediaDeletePermission = permission.MustDefinitionByCode("homepage_media.delete.any")
)

type Service struct {
	DB          *database.DB
	Redis       redisstore.Store
	CarouselDir string
}

type MediaValues struct {
	OverlayOpacityLight float64
	OverlayOpacityDark  float64
	StartYaw            float64
	StartPitch          float64
	YawSpeedDPS         float64
	PitchSpeedDPS       float64
	DurationMS          int
}

type PatchInput struct {
	Title               *string
	OverlayOpacityLight *float64
	OverlayOpacityDark  *float64
	StartYaw            *float64
	StartPitch          *float64
	YawSpeedDPS         *float64
	PitchSpeedDPS       *float64
	Enabled             *bool
	DurationMS          *int
}

func (s Service) List(ctx context.Context, actor permission.Actor) ([]model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaReadPermission); err != nil {
		return nil, err
	}
	items, err := s.DB.HomepageMedia.List(ctx, false)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	return items, nil
}

func (s Service) Patch(ctx context.Context, actor permission.Actor, id string, input PatchInput) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaUpdatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	if input.DurationMS != nil && (*input.DurationMS < 1000 || *input.DurationMS > 60000) {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Object: "homepage_duration", Operation: "validate", Reason: "out_of_range"}
	}
	if err := ValidateOpacity("overlay_opacity_light", input.OverlayOpacityLight); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := ValidateOpacity("overlay_opacity_dark", input.OverlayOpacityDark); err != nil {
		return model.HomepageMedia{}, err
	}
	item, err := s.DB.HomepageMedia.Get(ctx, id)
	if err != nil {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusNotFound, Object: "homepage_media", Operation: "resolve", Reason: "not_found"}
	}
	patch := dbhomepage.Patch{
		Title:               input.Title,
		OverlayOpacityLight: input.OverlayOpacityLight,
		OverlayOpacityDark:  input.OverlayOpacityDark,
		Enabled:             input.Enabled,
		DurationMS:          input.DurationMS,
		UpdatedAt:           database.NowMS(),
	}
	if item.Type == "panorama" {
		if err := ValidatePanoramaValues(input.StartYaw, input.StartPitch, input.YawSpeedDPS, input.PitchSpeedDPS); err != nil {
			return model.HomepageMedia{}, err
		}
		patch.StartYaw = input.StartYaw
		patch.StartPitch = input.StartPitch
		patch.YawSpeedDPS = input.YawSpeedDPS
		patch.PitchSpeedDPS = input.PitchSpeedDPS
	}
	item, err = s.DB.HomepageMedia.Patch(ctx, id, patch)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) Reorder(ctx context.Context, actor permission.Actor, ids []string) error {
	if err := requirePermission(actor, homepageMediaUpdatePermission); err != nil {
		return err
	}
	if len(ids) == 0 {
		return util.HTTPError{Status: http.StatusBadRequest, Object: "id_list", Operation: "validate", Reason: "required"}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return util.HTTPError{Status: http.StatusBadRequest, Object: "id_list", Operation: "validate", Reason: "invalid"}
		}
		seen[id] = true
	}
	if err := s.DB.HomepageMedia.Reorder(ctx, ids, database.NowMS()); err != nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "homepage_media", Operation: "resolve", Reason: "not_found"}
	}
	return s.Redis.InvalidatePublicHomepageMedia(ctx)
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, id string) error {
	if err := requirePermission(actor, homepageMediaDeletePermission); err != nil {
		return err
	}
	item, err := s.DB.HomepageMedia.Delete(ctx, id)
	if err != nil {
		return util.HTTPError{Status: http.StatusNotFound, Object: "homepage_media", Operation: "resolve", Reason: "not_found"}
	}
	path := filepath.Join(s.CarouselDir, item.StoragePath)
	if item.Type == "panorama" || strings.Contains(item.StoragePath, "/") {
		path = filepath.Join(s.CarouselDir, item.ID)
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return s.Redis.InvalidatePublicHomepageMedia(ctx)
}

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
