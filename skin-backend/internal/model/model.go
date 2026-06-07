package model

type User struct {
	ID                string
	Email             string
	Password          string
	IsAdmin           bool
	PreferredLanguage string
	DisplayName       string
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
