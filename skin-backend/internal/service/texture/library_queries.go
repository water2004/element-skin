package texture

import (
	"context"
	"net/http"
	"strings"

	texturedb "element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s LibraryService) PublicLibrary(ctx context.Context, actor permission.Actor, cursor string, limit int, typ, q, sort string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadPublicPermission); err != nil {
		return nil, err
	}
	enabled, err := s.Settings.Get(ctx, "enable_skin_library", "true")
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, util.HTTPError{Status: http.StatusForbidden, Object: "texture_library", Operation: "read", Reason: "disabled"}
	}
	lastCreated, lastHash, lastUsage, err := publicLibraryCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	parsedSort := texturedb.ParsePublicLibrarySort(sort)
	if cursor != "" && (lastCreated == nil || lastHash == "" ||
		(parsedSort == texturedb.PublicLibrarySortMostUsed && lastUsage == nil)) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	return s.DB.Textures.ListPublic(ctx, texturedb.PublicListOptions{
		Limit:       limit,
		TextureType: typ,
		Query:       strings.TrimSpace(q),
		Sort:        parsedSort,
		LastCreated: lastCreated,
		LastHash:    lastHash,
		LastUsage:   lastUsage,
	})
}

func (s LibraryService) ListMyTextures(ctx context.Context, actor permission.Actor, cursor string, limit int, typ string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadOwnedPermission); err != nil {
		return nil, err
	}
	lastCreated, lastHash, err := textureCursor(cursor, "last_hash")
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	if cursor != "" && (lastCreated == nil || lastHash == "") {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Object: "pagination_cursor", Operation: "decode", Reason: "invalid"}
	}
	return s.DB.Textures.ListForUser(ctx, actor.UserID, typ, limit, lastCreated, lastHash)
}

func (s LibraryService) TextureDetail(ctx context.Context, actor permission.Actor, hash, textureType string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadOwnedPermission); err != nil {
		return nil, err
	}
	info, err := s.DB.Textures.GetInfo(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Object: "texture", Operation: "resolve", Reason: "not_found"}
	}
	return info, nil
}
