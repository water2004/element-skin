package officialprofile

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	officialstore "element-skin/backend/internal/database/officialprofile"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

type preparedTexture struct {
	hash    *string
	created bool
}

type preparedRemoteTextures struct {
	storage *texturesvc.TextureStorage
	skin    preparedTexture
	cape    preparedTexture
}

func (s Service) Sync(ctx context.Context, actor permission.Actor, id string) (map[string]any, error) {
	if err := actor.Require(officialRefreshPermission); err != nil {
		return nil, forbidden()
	}
	id = strings.TrimSpace(id)
	current, err := s.DB.OfficialProfiles.GetByIDAndUser(ctx, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, notFound("official profile binding not found")
	}
	_, remote, err := s.resolve(ctx, actor.UserID, current.Binding.IdentityID)
	if err != nil {
		return nil, err
	}
	remoteUUID, err := normalizeRemoteProfile(*remote)
	if err != nil {
		return nil, err
	}
	if remoteUUID != current.Binding.RemoteUUID {
		return nil, conflict("Microsoft profile no longer matches this binding")
	}
	skinURL, capeURL, skinModel := remoteTextureMetadata(*remote)
	prepared, err := s.prepareRemoteTextures(ctx, skinURL, capeURL)
	if err != nil {
		return nil, err
	}

	now := database.NowMS()
	updated, err := s.DB.OfficialProfiles.Sync(ctx, officialstore.SyncInput{
		ID: id, UserID: actor.UserID, RemoteName: remote.Name,
		RemoteSkinURL: skinURL, RemoteCapeURL: capeURL, RemoteSkinModel: skinModel,
		SkinHash: prepared.skin.hash, CapeHash: prepared.cape.hash, SyncedAt: now,
	})
	if err != nil {
		s.cleanupPreparedTextures(ctx, prepared)
		return nil, err
	}
	if !updated {
		s.cleanupPreparedTextures(ctx, prepared)
		return nil, notFound("official profile binding not found")
	}
	item, err := s.DB.OfficialProfiles.GetByIDAndUser(ctx, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, notFound("official profile binding not found")
	}
	return bindingResponse(*item), nil
}

func (s Service) prepareRemoteTextures(ctx context.Context, skinURL, capeURL string) (preparedRemoteTextures, error) {
	if strings.TrimSpace(skinURL) == "" && strings.TrimSpace(capeURL) == "" {
		return preparedRemoteTextures{}, nil
	}
	storage, err := texturesvc.NewTextureStorage(s.TexturesDir)
	if err != nil {
		return preparedRemoteTextures{}, err
	}
	prepared := preparedRemoteTextures{storage: storage}
	prepared.skin, err = s.prepareTexture(ctx, storage, skinURL, "skin")
	if err != nil {
		s.cleanupPreparedTextures(ctx, prepared)
		return preparedRemoteTextures{}, err
	}
	prepared.cape, err = s.prepareTexture(ctx, storage, capeURL, "cape")
	if err != nil {
		s.cleanupPreparedTextures(ctx, prepared)
		return preparedRemoteTextures{}, err
	}
	return prepared, nil
}

func (s Service) cleanupPreparedTextures(ctx context.Context, prepared preparedRemoteTextures) {
	if prepared.storage == nil {
		return
	}
	for _, texture := range []preparedTexture{prepared.skin, prepared.cape} {
		if texture.hash == nil || !texture.created {
			continue
		}
		if inUse, err := s.DB.Textures.ExistsHash(ctx, *texture.hash); err == nil && !inUse {
			_ = prepared.storage.DeleteFile(*texture.hash)
		}
	}
}

func (s Service) prepareTexture(ctx context.Context, storage *texturesvc.TextureStorage, rawURL, textureType string) (preparedTexture, error) {
	if strings.TrimSpace(rawURL) == "" {
		return preparedTexture{}, nil
	}
	var data []byte
	var err error
	if s.Download != nil {
		data, err = s.Download(ctx, rawURL)
	} else {
		data, err = util.DownloadTexture(s.HTTPClient, rawURL, util.HardCapBytes)
	}
	if err != nil {
		return preparedTexture{}, util.HTTPError{Status: http.StatusBadGateway, Detail: "failed to download Microsoft profile texture"}
	}
	hash, created, err := storage.ProcessAndSaveTracked(data, textureType)
	if err != nil {
		return preparedTexture{}, util.HTTPError{Status: http.StatusBadGateway, Detail: "Microsoft profile texture is invalid"}
	}
	return preparedTexture{hash: &hash, created: created}, nil
}
