package microsoft

import "context"

type ProfileClient interface {
	AuthenticateXBL(ctx context.Context, msAccessToken string) (token string, userHash string, err error)
	AuthenticateXSTS(ctx context.Context, xblToken string) (token string, userHash string, err error)
	AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error)
	CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error)
	GetMinecraftProfile(ctx context.Context, mcAccessToken string) (*MinecraftProfile, error)
}

type ProfileResult struct {
	HasGame bool
	Profile *MinecraftProfile
}

type ProfileFlow struct {
	Client ProfileClient
}

func (f ProfileFlow) Resolve(ctx context.Context, microsoftAccessToken string) (ProfileResult, error) {
	xblToken, _, err := f.Client.AuthenticateXBL(ctx, microsoftAccessToken)
	if err != nil {
		return ProfileResult{}, err
	}
	xstsToken, userHash, err := f.Client.AuthenticateXSTS(ctx, xblToken)
	if err != nil {
		return ProfileResult{}, err
	}
	minecraftAccessToken, err := f.Client.AuthenticateMinecraft(ctx, userHash, xstsToken)
	if err != nil {
		return ProfileResult{}, err
	}
	hasGame, err := f.Client.CheckGameOwnership(ctx, minecraftAccessToken)
	if err != nil {
		return ProfileResult{}, err
	}
	if !hasGame {
		return ProfileResult{HasGame: false}, nil
	}
	profile, err := f.Client.GetMinecraftProfile(ctx, minecraftAccessToken)
	if err != nil {
		return ProfileResult{}, err
	}
	return ProfileResult{HasGame: true, Profile: profile}, nil
}
