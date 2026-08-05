package imports

import (
	"context"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	microsoftsvc "element-skin/backend/internal/service/microsoft"
	"element-skin/backend/internal/util"
)

const (
	microsoftOAuthStateTTL   = 10 * time.Minute
	microsoftProfileStateTTL = 5 * time.Minute
	microsoftImportStateTTL  = 5 * time.Minute
)

var (
	microsoftImportStartPermission   = permission.MustDefinitionByCode("microsoft_import.start.owned")
	microsoftReadProfilePermission   = permission.MustDefinitionByCode("microsoft_import.read_profile.owned")
	microsoftCreateProfilePermission = permission.MustDefinitionByCode("microsoft_import.create_profile.owned")
)

type MicrosoftSettings interface {
	Get(context.Context, string, string) (string, error)
}

type MicrosoftImportWorkflow struct {
	APIURL     string
	SiteURL    string
	Settings   MicrosoftSettings
	States     redisstore.Store
	Profiles   ImportService
	HTTPClient *http.Client
}

type MicrosoftAuthStart struct {
	AuthorizationURL string
	State            string
}

type MicrosoftProfilePreview struct {
	Profile     map[string]any
	HasGame     any
	ImportToken string
}

func (s MicrosoftImportWorkflow) Start(ctx context.Context, actor permission.Actor) (MicrosoftAuthStart, error) {
	if !actor.Has(microsoftImportStartPermission) {
		return MicrosoftAuthStart{}, util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
	}
	clientID, err := s.Settings.Get(ctx, "microsoft_client_id", "")
	if err != nil {
		return MicrosoftAuthStart{}, err
	}
	redirectURI, err := s.redirectURI(ctx)
	if err != nil {
		return MicrosoftAuthStart{}, err
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(redirectURI) == "" {
		return MicrosoftAuthStart{}, util.HTTPError{Status: http.StatusServiceUnavailable, Detail: "Microsoft import is not configured"}
	}
	state, err := randomMicrosoftToken(64)
	if err != nil {
		return MicrosoftAuthStart{}, err
	}
	if err := s.States.SetState(ctx, state, map[string]any{
		"user_id": actor.UserID,
		"kind":    microsoftStateKindOAuth,
	}, microsoftOAuthStateTTL); err != nil {
		return MicrosoftAuthStart{}, err
	}
	return MicrosoftAuthStart{
		AuthorizationURL: microsoftsvc.MicrosoftAuthorizationURL(clientID, redirectURI, state),
		State:            state,
	}, nil
}

func (s MicrosoftImportWorkflow) Complete(ctx context.Context, code, state string) (string, error) {
	session, err := s.popState(ctx, state, microsoftStateKindOAuth, "Invalid or expired state parameter")
	if err != nil {
		return "", err
	}
	clientID, err := s.Settings.Get(ctx, "microsoft_client_id", "")
	if err != nil {
		return "", err
	}
	clientSecret, err := s.Settings.Get(ctx, "microsoft_client_secret", "")
	if err != nil {
		return "", err
	}
	redirectURI, err := s.redirectURI(ctx)
	if err != nil {
		return "", err
	}
	failureURL, err := s.callbackURL("error=auth_failed")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || strings.TrimSpace(redirectURI) == "" {
		return failureURL, nil
	}
	result, err := (microsoftsvc.MicrosoftAuthFlow{Client: microsoftsvc.MicrosoftHTTPClient{
		Client:       s.HTTPClient,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
	}}).Complete(ctx, code)
	if err != nil || result["profile"] == nil {
		return failureURL, nil
	}
	token, err := randomMicrosoftToken(64)
	if err != nil {
		return "", err
	}
	if err := s.States.SetState(ctx, token, map[string]any{
		"user_id": session["user_id"],
		"kind":    microsoftStateKindProfile,
		"profile": result,
	}, microsoftProfileStateTTL); err != nil {
		return "", err
	}
	return s.callbackURL("ms_token=" + token)
}

func (s MicrosoftImportWorkflow) Preview(ctx context.Context, actor permission.Actor, token string) (MicrosoftProfilePreview, error) {
	if !actor.Has(microsoftReadProfilePermission) {
		return MicrosoftProfilePreview{}, util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
	}
	session, err := s.popState(ctx, token, microsoftStateKindProfile, "Invalid or expired token")
	if err != nil {
		return MicrosoftProfilePreview{}, err
	}
	if err := requireMicrosoftStateOwner(session, actor.UserID, "Unauthorized"); err != nil {
		return MicrosoftProfilePreview{}, err
	}
	flowProfile, _ := session["profile"].(map[string]any)
	verified := verifiedMicrosoftProfile(flowProfile)
	importToken, err := randomMicrosoftToken(64)
	if err != nil {
		return MicrosoftProfilePreview{}, err
	}
	if err := s.States.SetState(ctx, importToken, map[string]any{
		"user_id": actor.UserID,
		"kind":    microsoftStateKindImport,
		"profile": verified,
	}, microsoftImportStateTTL); err != nil {
		return MicrosoftProfilePreview{}, err
	}
	return MicrosoftProfilePreview{
		Profile:     verified,
		HasGame:     valueOrAny(flowProfile["has_game"], false),
		ImportToken: importToken,
	}, nil
}

func (s MicrosoftImportWorkflow) Import(ctx context.Context, actor permission.Actor, token string) (map[string]any, error) {
	if !actor.Has(microsoftCreateProfilePermission) {
		return nil, util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
	}
	session, err := s.popState(ctx, token, microsoftStateKindImport, "invalid import token")
	if err != nil {
		return nil, err
	}
	if err := requireMicrosoftStateOwner(session, actor.UserID, "not allowed"); err != nil {
		return nil, err
	}
	profile, ok := session["profile"].(map[string]any)
	if !ok {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid import token"}
	}
	profileID, _ := profile["id"].(string)
	profileName, _ := profile["name"].(string)
	return s.Profiles.ImportProfile(ctx, actor, profileID, profileName, microsoftProfileAssets(profile))
}

func (s MicrosoftImportWorkflow) redirectURI(ctx context.Context) (string, error) {
	fallback := strings.TrimRight(s.APIURL, "/") + "/v2/imports/microsoft/callback"
	return s.Settings.Get(ctx, "microsoft_redirect_uri", fallback)
}

func (s MicrosoftImportWorkflow) callbackURL(query string) (string, error) {
	siteURL := strings.TrimRight(strings.TrimSpace(s.SiteURL), "/")
	if siteURL == "" {
		return "", util.HTTPError{Status: http.StatusInternalServerError, Detail: "site URL is not configured"}
	}
	return siteURL + "/dashboard/roles?" + query, nil
}

func valueOrAny(value, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}
