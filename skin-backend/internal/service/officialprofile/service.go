package officialprofile

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	officialstore "element-skin/backend/internal/database/officialprofile"
	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	identitysvc "element-skin/backend/internal/service/identity"
	microsoftsvc "element-skin/backend/internal/service/microsoft"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	officialReadPermission    = permission.MustDefinitionByCode("official_profile.read.owned")
	officialCreatePermission  = permission.MustDefinitionByCode("official_profile.create.owned")
	officialRefreshPermission = permission.MustDefinitionByCode("official_profile.refresh.owned")
	officialDeletePermission  = permission.MustDefinitionByCode("official_profile.delete.owned")
)

type MicrosoftResolver interface {
	Resolve(context.Context, string) (microsoftsvc.ProfileResult, error)
}

type Service struct {
	DB          *database.DB
	Identities  identitysvc.Service
	Resolver    MicrosoftResolver
	TexturesDir string
	HTTPClient  *http.Client
	Download    func(context.Context, string) ([]byte, error)
}

func (s Service) List(ctx context.Context, actor permission.Actor) ([]map[string]any, error) {
	if err := actor.Require(officialReadPermission); err != nil {
		return nil, forbidden()
	}
	items, err := s.DB.OfficialProfiles.ListByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, bindingResponse(item))
	}
	return out, nil
}

func (s Service) Create(ctx context.Context, actor permission.Actor, identityID string) (map[string]any, error) {
	if err := actor.Require(officialCreatePermission); err != nil {
		return nil, forbidden()
	}
	identityID = strings.TrimSpace(identityID)
	if identityID == "" {
		return nil, badRequest("identity_id is required")
	}
	access, remote, err := s.resolve(ctx, actor.UserID, identityID)
	if err != nil {
		return nil, err
	}
	remoteUUID, err := normalizeRemoteProfile(*remote)
	if err != nil {
		return nil, err
	}
	skinURL, capeURL, skinModel := remoteTextureMetadata(*remote)
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	now := database.NowMS()
	binding := model.OfficialProfileBinding{
		ID: id, IdentityID: access.Identity.ID, ProfileID: remoteUUID,
		RemoteUUID: remoteUUID, RemoteName: remote.Name, RemoteSkinURL: skinURL,
		RemoteCapeURL: capeURL, RemoteSkinModel: skinModel, CreatedAt: now, UpdatedAt: now,
	}
	profileNames := make([]string, 100)
	for attempt := range profileNames {
		profileNames[attempt] = util.ProfileNameCandidate(remote.Name, attempt)
	}
	if err := s.DB.OfficialProfiles.Create(ctx, officialstore.CreateInput{
		Binding: binding, UserID: actor.UserID, ProfileNames: profileNames, ProfileModel: skinModel,
	}); err != nil {
		if errors.Is(err, officialstore.ErrProfileOwnedByAnotherUser) {
			return nil, conflict("official profile UUID belongs to another user")
		}
		if errors.Is(err, officialstore.ErrProfileNameUnavailable) {
			return nil, conflict("unable to allocate profile name")
		}
		if constraint := uniqueConstraint(err); constraint == "official_profile_bindings_profile_id_key" ||
			constraint == "idx_official_profile_bindings_remote_uuid" {
			return nil, conflict("official profile is already bound")
		}
		return nil, err
	}
	created, err := s.DB.OfficialProfiles.GetByIDAndUser(ctx, binding.ID, actor.UserID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, errors.New("created official profile binding is missing")
	}
	return bindingResponse(*created), nil
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, id string) error {
	if err := actor.Require(officialDeletePermission); err != nil {
		return forbidden()
	}
	deleted, err := s.DB.OfficialProfiles.DeleteByIDAndUser(ctx, strings.TrimSpace(id), actor.UserID)
	if err != nil {
		return err
	}
	if !deleted {
		return notFound("official profile binding not found")
	}
	return nil
}

func (s Service) resolve(ctx context.Context, userID, identityID string) (identitysvc.AuthorizedIdentity, *microsoftsvc.MinecraftProfile, error) {
	identity, err := s.DB.Identities.GetIdentity(ctx, strings.TrimSpace(identityID))
	if err != nil {
		return identitysvc.AuthorizedIdentity{}, nil, err
	}
	if identity == nil || identity.UserID != userID {
		return identitysvc.AuthorizedIdentity{}, nil, notFound("external identity not found")
	}
	provider, err := s.DB.Identities.GetProvider(ctx, identity.ProviderID)
	if err != nil {
		return identitysvc.AuthorizedIdentity{}, nil, err
	}
	if provider == nil || provider.Adapter != identitysvc.AdapterMicrosoft {
		return identitysvc.AuthorizedIdentity{}, nil, conflict("external identity is not a Microsoft identity")
	}
	access, err := s.Identities.AccessTokenForOwnedIdentity(ctx, userID, identityID)
	if err != nil {
		return identitysvc.AuthorizedIdentity{}, nil, err
	}
	resolver := s.Resolver
	if resolver == nil {
		resolver = microsoftsvc.ProfileFlow{Client: microsoftsvc.MicrosoftHTTPClient{Client: s.HTTPClient}}
	}
	result, err := resolver.Resolve(ctx, access.AccessToken)
	if microsoftsvc.IsUnauthorized(err) {
		access, err = s.Identities.ForceRefreshAccessTokenForOwnedIdentity(ctx, userID, identityID)
		if err != nil {
			return identitysvc.AuthorizedIdentity{}, nil, err
		}
		result, err = resolver.Resolve(ctx, access.AccessToken)
	}
	if err != nil {
		return identitysvc.AuthorizedIdentity{}, nil, util.HTTPError{Status: http.StatusBadGateway, Detail: "Microsoft profile request failed"}
	}
	if !result.HasGame || result.Profile == nil {
		return identitysvc.AuthorizedIdentity{}, nil, conflict("Microsoft identity does not own Minecraft: Java Edition")
	}
	return access, result.Profile, nil
}

func normalizeRemoteProfile(profile microsoftsvc.MinecraftProfile) (string, error) {
	id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(profile.ID), "-", ""))
	if decoded, err := hex.DecodeString(id); err != nil || len(decoded) != 16 {
		return "", util.HTTPError{Status: http.StatusBadGateway, Detail: "Microsoft profile response is invalid"}
	}
	if !util.ValidProfileName(profile.Name) {
		return "", util.HTTPError{Status: http.StatusBadGateway, Detail: "Microsoft profile response is invalid"}
	}
	return id, nil
}

func remoteTextureMetadata(profile microsoftsvc.MinecraftProfile) (skinURL, capeURL, skinModel string) {
	skinModel = "default"
	if skin := profile.ActiveSkin(); skin != nil {
		skinURL = strings.TrimSpace(skin.URL)
		if strings.EqualFold(skin.Variant, "slim") {
			skinModel = "slim"
		}
	}
	if cape := profile.ActiveCape(); cape != nil {
		capeURL = strings.TrimSpace(cape.URL)
	}
	return skinURL, capeURL, skinModel
}

func bindingResponse(item officialstore.View) map[string]any {
	return map[string]any{
		"id":                item.Binding.ID,
		"identity_id":       item.Binding.IdentityID,
		"profile_id":        item.Binding.ProfileID,
		"remote_uuid":       item.Binding.RemoteUUID,
		"remote_name":       item.Binding.RemoteName,
		"remote_skin_url":   item.Binding.RemoteSkinURL,
		"remote_cape_url":   item.Binding.RemoteCapeURL,
		"remote_skin_model": item.Binding.RemoteSkinModel,
		"created_at":        item.Binding.CreatedAt,
		"updated_at":        item.Binding.UpdatedAt,
		"last_synced_at":    item.Binding.LastSyncedAt,
		"profile":           profilestore.Summary(item.Profile),
		"identity": map[string]any{
			"id":               item.Binding.IdentityID,
			"label":            item.IdentityLabel,
			"provider_id":      item.ProviderID,
			"provider_name":    item.ProviderName,
			"provider_adapter": item.ProviderAdapter,
		},
	}
}

func uniqueConstraint(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName
	}
	return ""
}

func badRequest(detail string) util.HTTPError { return util.HTTPError{Status: 400, Detail: detail} }
func forbidden() util.HTTPError               { return util.HTTPError{Status: 403, Detail: "permission denied"} }
func notFound(detail string) util.HTTPError   { return util.HTTPError{Status: 404, Detail: detail} }
func conflict(detail string) util.HTTPError   { return util.HTTPError{Status: 409, Detail: detail} }
