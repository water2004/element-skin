package oauth

import (
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
)

const (
	ClientTypePublic       = "public"
	ClientTypeConfidential = "confidential"
	StatusPending          = "pending"
	StatusActive           = "active"
	StatusRejected         = "rejected"
	StatusDisabled         = "disabled"

	authorizationCodeTTL = 10 * time.Minute
	deviceCodeTTL        = 10 * time.Minute
	devicePollInterval   = 5 * time.Second
	accessTokenTTL       = time.Hour
	refreshTokenTTL      = 30 * 24 * time.Hour
)

type Service struct {
	DB    *database.DB
	Redis redisstore.Store
}

type ClientInput struct {
	Name            string
	Description     string
	RedirectURI     string
	WebsiteURL      string
	ClientType      string
	PermissionCodes []string
}

type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type AuthorizationDetails struct {
	Client      map[string]any   `json:"client"`
	Scopes      []map[string]any `json:"scopes"`
	RedirectURI string           `json:"redirect_uri"`
	State       string           `json:"state,omitempty"`
}

type DeviceAuthorizationRequest struct {
	ClientID     string
	ClientSecret string
	Scope        string
}

type DeviceAuthorizationResponse struct {
	DeviceCode  string   `json:"device_code"`
	UserCode    string   `json:"user_code"`
	ExpiresIn   int64    `json:"expires_in"`
	Interval    int64    `json:"interval"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

type DeviceAuthorizationDetails struct {
	Client    map[string]any   `json:"client"`
	Scopes    []map[string]any `json:"scopes"`
	ExpiresAt int64            `json:"expires_at"`
	Status    string           `json:"status"`
}

type DeviceDecisionRequest struct {
	UserCode string
	Approve  bool
}

type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
	RefreshToken string
	Scope        string
	DeviceCode   string
}

type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int64    `json:"expires_in"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	Scope        string   `json:"scope"`
	Permissions  []string `json:"permissions"`
}
