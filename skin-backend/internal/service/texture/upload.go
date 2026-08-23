package texture

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var (
	textureCreateOwnedPermission = permission.MustDefinitionByCode("texture.create.owned")
)

type UploadService struct {
	DB          *database.DB
	TexturesDir string
}

type UploadInput struct {
	Actor       permission.Actor
	Data        []byte
	TextureType string
	Note        string
	IsPublic    bool
	Model       string
}

func (s UploadService) UploadToLibrary(ctx context.Context, input UploadInput) (map[string]any, error) {
	if err := input.Actor.Require(textureCreateOwnedPermission); err != nil {
		return nil, permissionDenied()
	}
	textureType, err := normalizedUploadTextureType(input.TextureType, true)
	if err != nil {
		return nil, err
	}
	if input.IsPublic {
		if err := input.Actor.Require(textureUpdateVisibilityOwnedPermission); err != nil {
			return nil, permissionDenied()
		}
	}
	hash, err := s.saveLibraryTexture(ctx, input.Actor.UserID, input.Data, textureType, input.Note, input.IsPublic, input.Model)
	if err != nil {
		return nil, err
	}
	return map[string]any{"hash": hash, "type": textureType}, nil
}

func (s UploadService) UploadAndApply(ctx context.Context, input UploadInput, profileID string) (map[string]any, error) {
	if err := input.Actor.Require(textureCreateOwnedPermission); err != nil {
		return nil, permissionDenied()
	}
	profileID = strings.TrimSpace(profileID)
	textureType, err := normalizedUploadTextureType(input.TextureType, false)
	if err != nil {
		return nil, err
	}
	if profileID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "texture", Operation: "upload", Reason: "required"}
	}
	if err := requireOwnedOrBoundProfilePermission(input.Actor, profileID, textureApplyOwnedPermission, textureApplyBoundPermission); err != nil {
		return nil, err
	}
	if input.IsPublic {
		if err := input.Actor.Require(textureUpdateVisibilityOwnedPermission); err != nil {
			return nil, permissionDenied()
		}
	}
	model := profile.NormalizeModel(input.Model)
	hash, err := s.saveLibraryTexture(ctx, input.Actor.UserID, input.Data, textureType, "", input.IsPublic, model)
	if err != nil {
		return nil, err
	}
	if err := s.applyUploadedTexture(ctx, input.Actor.UserID, profileID, hash, textureType, model); err != nil {
		return nil, err
	}
	return map[string]any{"hash": hash, "type": textureType}, nil
}

func (s UploadService) UploadAndApplyBoundProfile(ctx context.Context, input UploadInput, profileID string) (map[string]any, error) {
	profileID = strings.TrimSpace(profileID)
	textureType, err := normalizedUploadTextureType(input.TextureType, false)
	if err != nil {
		return nil, err
	}
	if profileID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "texture", Operation: "upload", Reason: "required"}
	}
	if err := requireBoundProfilePermission(input.Actor, profileID, textureApplyBoundPermission); err != nil {
		return nil, err
	}
	model := profile.NormalizeModel(input.Model)
	hash, err := s.saveLibraryTexture(ctx, input.Actor.UserID, input.Data, textureType, "", false, model)
	if err != nil {
		return nil, err
	}
	if err := s.applyUploadedTexture(ctx, input.Actor.UserID, profileID, hash, textureType, model); err != nil {
		return nil, err
	}
	return map[string]any{"hash": hash, "type": textureType}, nil
}

func (s UploadService) saveLibraryTexture(ctx context.Context, userID string, data []byte, textureType, note string, isPublic bool, model string) (string, error) {
	storage, err := NewTextureStorage(s.TexturesDir)
	if err != nil {
		return "", err
	}
	hash, created, err := storage.ProcessAndSaveTracked(data, textureType)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPNG):
			return "", util.HTTPError{Status: http.StatusBadRequest, Object: "texture_format", Operation: "validate", Reason: "unsupported"}
		case errors.Is(err, ErrInvalidTextureDimensions):
			return "", util.HTTPError{Status: http.StatusBadRequest, Object: "texture_dimensions", Operation: "validate", Reason: "invalid"}
		default:
			return "", err
		}
	}
	if err := s.DB.Textures.AddToLibrary(ctx, userID, hash, textureType, note, isPublic, profile.NormalizeModel(model)); err != nil {
		if created {
			if inUse, checkErr := s.DB.Textures.ExistsHash(ctx, hash); checkErr == nil && !inUse {
				_ = storage.DeleteFile(hash)
			}
		}
		return "", err
	}
	return hash, nil
}

func (s UploadService) applyUploadedTexture(ctx context.Context, userID, profileID, hash, textureType, model string) error {
	owns, err := s.DB.Textures.VerifyOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: http.StatusForbidden, Object: "texture", Operation: "authorize", Reason: "denied"}
	}
	profileOwner, err := s.DB.Profiles.VerifyOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: http.StatusForbidden, Object: "profile", Operation: "authorize", Reason: "denied"}
	}
	switch textureType {
	case "skin":
		return s.DB.Profiles.UpdateSkinAndModel(ctx, profileID, &hash, model)
	case "cape":
		return s.DB.Profiles.UpdateCape(ctx, profileID, &hash)
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
}

func normalizedUploadTextureType(raw string, defaultSkin bool) (string, error) {
	textureType := strings.ToLower(strings.TrimSpace(raw))
	if textureType == "" && defaultSkin {
		textureType = "skin"
	}
	if textureType == "" {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "texture", Operation: "upload", Reason: "required"}
	}
	if textureType != "skin" && textureType != "cape" {
		return "", util.HTTPError{Status: http.StatusBadRequest, Object: "texture_type", Operation: "validate", Reason: "invalid"}
	}
	return textureType, nil
}

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
}
