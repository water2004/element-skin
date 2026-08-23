package microsoft

import (
	"context"
	"fmt"
	"strings"
)

type MinecraftTexture struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	URL     string `json:"url"`
	Variant string `json:"variant"`
	Alias   string `json:"alias"`
}

type MinecraftProfile struct {
	ID    string             `json:"id"`
	Name  string             `json:"name"`
	Skins []MinecraftTexture `json:"skins"`
	Capes []MinecraftTexture `json:"capes"`
}

func (p MinecraftProfile) ActiveSkin() *MinecraftTexture {
	return activeTexture(p.Skins)
}

func (p MinecraftProfile) ActiveCape() *MinecraftTexture {
	return activeTexture(p.Capes)
}

func activeTexture(items []MinecraftTexture) *MinecraftTexture {
	for i := range items {
		if strings.EqualFold(items[i].State, "active") && strings.TrimSpace(items[i].URL) != "" {
			return &items[i]
		}
	}
	for i := range items {
		if strings.TrimSpace(items[i].URL) != "" {
			return &items[i]
		}
	}
	return nil
}

func (c MicrosoftHTTPClient) AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error) {
	var out map[string]any
	if err := c.postJSON(ctx, "https://api.minecraftservices.com/authentication/login_with_xbox", map[string]any{
		"identityToken": "XBL3.0 x=" + userHash + ";" + xstsToken,
	}, "", &out); err != nil {
		return "", err
	}
	token, _ := out["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("minecraft login response missing access_token")
	}
	return token, nil
}

func (c MicrosoftHTTPClient) CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error) {
	var out struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/entitlements/mcstore", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return false, err
	}
	return len(out.Items) > 0, nil
}

func (c MicrosoftHTTPClient) GetMinecraftProfile(ctx context.Context, mcAccessToken string) (*MinecraftProfile, error) {
	var out *MinecraftProfile
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/minecraft/profile", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return nil, err
	}
	return out, nil
}
