package yggdrasil

import (
	"context"
	"net/http"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/util"
)

var sitePublicReadPermission = permission.MustDefinitionByCode("site_public.read.public")

func (y Yggdrasil) PublicKeys(ctx context.Context, actor permission.Actor) (model.YggdrasilPublicKeys, error) {
	if err := requirePublicPermission(actor); err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	signer, err := y.signer()
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	own, err := fallbacksvc.NormalizePEMPublicKeys([]string{signer.PublicKeyPEM()})
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	groups, err := y.cachedFallbackPublicKeys(ctx)
	if err != nil {
		return model.YggdrasilPublicKeys{}, err
	}
	groups = append([]model.YggdrasilPublicKeys{{
		PlayerCertificateKeys: append([]model.YggdrasilPublicKey(nil), own...),
		ProfilePropertyKeys:   append([]model.YggdrasilPublicKey(nil), own...),
	}}, groups...)
	return fallbacksvc.MergePublicKeys(groups...), nil
}

func (y Yggdrasil) cachedFallbackPublicKeys(ctx context.Context) ([]model.YggdrasilPublicKeys, error) {
	endpoints, err := y.DB.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	sources := fallbacksvc.PublicKeySources(endpoints)
	if len(sources) == 0 {
		return nil, nil
	}
	cache := y.Redis
	if cache == nil {
		cache = y.Settings.Redis
	}
	if cache == nil {
		return nil, redisstore.ErrCacheMiss
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.ID)
	}
	cached, err := cache.GetFallbackPublicKeys(ctx, ids)
	if err != nil {
		return nil, err
	}
	groups := make([]model.YggdrasilPublicKeys, 0, len(cached))
	for _, source := range sources {
		if keys, exists := cached[source.ID]; exists {
			groups = append(groups, keys)
		}
	}
	return groups, nil
}

func requirePublicPermission(actor permission.Actor) error {
	if err := actor.Require(sitePublicReadPermission); err != nil {
		return util.HTTPError{Status: http.StatusForbidden, Object: "permission", Operation: "check", Reason: "denied"}
	}
	return nil
}
