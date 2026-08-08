package model

const (
	ExternalIdentityAuthorizationActive                  = "active"
	ExternalIdentityAuthorizationReauthorizationRequired = "reauthorization_required"
)

type User struct {
	ID                string
	Email             string
	Password          string
	PreferredLanguage string
	DisplayName       string
	CreatedAt         int64
	BannedUntil       *int64
	AvatarHash        *string
}

type Profile struct {
	ID           string
	UserID       string
	Name         string
	TextureModel string
	SkinHash     *string
	CapeHash     *string
}

type Token struct {
	AccessToken string
	ClientToken string
	UserID      string
	ProfileID   *string
	CreatedAt   int64
}

type Session struct {
	ServerID    string
	AccessToken string
	IP          *string
	CreatedAt   int64
}

type Invite struct {
	Code      string
	CreatedAt *int64
	UsedBy    *string
	TotalUses *int
	UsedCount int
	Note      string
}

type HomepageMedia struct {
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	Title               string  `json:"title"`
	StoragePath         string  `json:"storage_path"`
	OverlayOpacityLight float64 `json:"overlay_opacity_light"`
	OverlayOpacityDark  float64 `json:"overlay_opacity_dark"`
	StartYaw            float64 `json:"start_yaw"`
	StartPitch          float64 `json:"start_pitch"`
	YawSpeedDPS         float64 `json:"yaw_speed_dps"`
	PitchSpeedDPS       float64 `json:"pitch_speed_dps"`
	SortOrder           int     `json:"sort_order"`
	Enabled             bool    `json:"enabled"`
	DurationMS          int     `json:"duration_ms"`
	CreatedAt           int64   `json:"created_at"`
	UpdatedAt           int64   `json:"updated_at"`
}

type Notice struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	ContentMarkdown string  `json:"content_markdown"`
	DisplayMode     string  `json:"display_mode"`
	Level           string  `json:"level"`
	LinkText        string  `json:"link_text"`
	LinkURL         string  `json:"link_url"`
	Audience        string  `json:"audience"`
	Enabled         bool    `json:"enabled"`
	Pinned          bool    `json:"pinned"`
	Dismissible     bool    `json:"dismissible"`
	StartsAt        *int64  `json:"starts_at"`
	EndsAt          *int64  `json:"ends_at"`
	CreatedBy       *string `json:"created_by"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type NoticeView struct {
	Notice
	Read        bool   `json:"read"`
	ReadAt      *int64 `json:"read_at"`
	DismissedAt *int64 `json:"dismissed_at"`
}

type OAuthClient struct {
	ID          string `json:"client_id"`
	OwnerUserID string `json:"owner_user_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	RedirectURI string `json:"redirect_uri"`
	WebsiteURL  string `json:"website_url"`
	ClientType  string `json:"client_type"`
	SecretHash  string `json:"-"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type OAuthGrant struct {
	ID         string   `json:"id"`
	UserID     string   `json:"user_id"`
	SubjectID  string   `json:"subject_id"`
	ClientID   string   `json:"client_id"`
	OIDCScopes []string `json:"oidc_scopes"`
	Status     string   `json:"status"`
	CreatedAt  int64    `json:"created_at"`
	RevokedAt  *int64   `json:"revoked_at"`
}

type OAuthAuthorizationCode struct {
	CodeHash            string
	ClientID            string
	UserID              string
	GrantID             string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	OIDCScopes          []string
	Nonce               string
	ExpiresAt           int64
	CreatedAt           int64
	ConsumedAt          *int64
}

type OAuthToken struct {
	TokenHash  string
	ClientID   string
	UserID     string
	GrantID    string
	OIDCScopes []string
	ExpiresAt  int64
	CreatedAt  int64
	RevokedAt  *int64
}

type OAuthDeviceCode struct {
	DeviceCodeHash string
	UserCodeHash   string
	ClientID       string
	UserID         *string
	SubjectID      *string
	Status         string
	ExpiresAt      int64
	CreatedAt      int64
	ApprovedAt     *int64
	DeniedAt       *int64
	ConsumedAt     *int64
	LastPolledAt   *int64
}

type WebhookEndpoint struct {
	ID               string   `json:"id"`
	ClientID         string   `json:"client_id"`
	URL              string   `json:"url"`
	SecretCiphertext string   `json:"-"`
	Status           string   `json:"status"`
	EventTypes       []string `json:"events"`
	CreatedAt        int64    `json:"created_at"`
	UpdatedAt        int64    `json:"updated_at"`
}

type WebhookEvent struct {
	ID                  string         `json:"id"`
	Type                string         `json:"type"`
	TargetClientID      string         `json:"-"`
	SubjectUserID       string         `json:"-"`
	Data                map[string]any `json:"data"`
	CreatedAt           int64          `json:"created_at"`
	ExpandedAt          *int64         `json:"-"`
	ExpansionLeaseUntil *int64         `json:"-"`
	ExpansionLeaseToken string         `json:"-"`
}

type WebhookExpansion struct {
	EventID     string
	LeaseToken  string
	EndpointIDs []string
}

type WebhookDelivery struct {
	ID           string
	Event        WebhookEvent
	Endpoint     WebhookEndpoint
	AttemptCount int
	CreatedAt    int64
	LeaseToken   string
}

type WebhookDeliveryOutcome struct {
	DeliveryID    string
	LeaseToken    string
	Status        string
	NextAttemptAt int64
	UpdatedAt     int64
	HTTPStatus    *int
	Detail        string
	DeliveredAt   *int64
}

type IdentityProvider struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	IssuerURL              string   `json:"issuer_url"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	UserInfoEndpoint       string   `json:"userinfo_endpoint"`
	JWKSURI                string   `json:"jwks_uri"`
	ClientID               string   `json:"client_id"`
	ClientSecretCiphertext string   `json:"-"`
	Scopes                 []string `json:"scopes"`
	Adapter                string   `json:"adapter"`
	IconURL                string   `json:"icon_url"`
	Enabled                bool     `json:"enabled"`
	LoginEnabled           bool     `json:"login_enabled"`
	LinkEnabled            bool     `json:"link_enabled"`
	RegistrationEnabled    bool     `json:"registration_enabled"`
	DisplayOrder           int      `json:"display_order"`
	CreatedAt              int64    `json:"created_at"`
	UpdatedAt              int64    `json:"updated_at"`
}

type ExternalIdentity struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	ProviderID    string `json:"provider_id"`
	Subject       string `json:"subject"`
	Label         string `json:"label"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	LastLoginAt   *int64 `json:"last_login_at"`
}

type ExternalIdentityCredential struct {
	IdentityID             string
	RefreshTokenCiphertext string
	GrantedScopes          []string
	AuthorizationStatus    string
	LastRefreshAt          *int64
	LastRefreshErrorAt     *int64
	UpdatedAt              int64
}

type OfficialProfileBinding struct {
	ID              string `json:"id"`
	IdentityID      string `json:"identity_id"`
	ProfileID       string `json:"profile_id"`
	RemoteUUID      string `json:"remote_uuid"`
	RemoteName      string `json:"remote_name"`
	RemoteSkinURL   string `json:"remote_skin_url"`
	RemoteCapeURL   string `json:"remote_cape_url"`
	RemoteSkinModel string `json:"remote_skin_model"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	LastSyncedAt    *int64 `json:"last_synced_at"`
}
