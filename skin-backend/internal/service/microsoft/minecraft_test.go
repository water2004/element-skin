package microsoft_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

func TestMicrosoftHTTPClientMinecraftRequestBody(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body %s: %v", body, err)
		}
		if req.URL.String() != "https://api.minecraftservices.com/authentication/login_with_xbox" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if payload["identityToken"] != "XBL3.0 x=user_hash;xsts_token" {
			t.Fatalf("unexpected Minecraft payload: %#v", payload)
		}
		return jsonResponse(`{"access_token":"mc_access"}`), nil
	})}}

	mcToken, err := client.AuthenticateMinecraft(context.Background(), "user_hash", "xsts_token")
	if err != nil || mcToken != "mc_access" {
		t.Fatalf("minecraft got token=%q err=%v", mcToken, err)
	}
}

func TestMicrosoftHTTPClientRejectsMalformedMinecraftResponse(t *testing.T) {
	minecraftClient := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{}`), nil
	})}}
	if _, err := minecraftClient.AuthenticateMinecraft(context.Background(), "uhs", "xsts"); err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing minecraft access_token error, got %v", err)
	}
}

func TestMicrosoftHTTPClientOwnershipAndProfileContracts(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer mc_access" {
			t.Fatalf("authorization header=%q, want Bearer mc_access", req.Header.Get("Authorization"))
		}
		switch req.URL.String() {
		case "https://api.minecraftservices.com/entitlements/mcstore":
			if req.Method != http.MethodGet {
				t.Fatalf("ownership method=%s, want GET", req.Method)
			}
			return jsonResponse(`{"items":[{"name":"game_minecraft"}]}`), nil
		case "https://api.minecraftservices.com/minecraft/profile":
			if req.Method != http.MethodGet {
				t.Fatalf("profile method=%s, want GET", req.Method)
			}
			return jsonResponse(`{"id":"profile-id","name":"Steve","skins":[],"capes":[]}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})}}

	owned, err := client.CheckGameOwnership(context.Background(), "mc_access")
	if err != nil || !owned {
		t.Fatalf("ownership result=%v err=%v, want true nil", owned, err)
	}
	profile, err := client.GetMinecraftProfile(context.Background(), "mc_access")
	if err != nil || profile == nil || profile.ID != "profile-id" || profile.Name != "Steve" || len(profile.Skins) != 0 || len(profile.Capes) != 0 {
		t.Fatalf("profile=%#v err=%v", profile, err)
	}
}

func TestMinecraftProfileSelectsActiveTexturesExactly(t *testing.T) {
	profile := microsoft.MinecraftProfile{
		Skins: []microsoft.MinecraftTexture{
			{ID: "inactive", State: "INACTIVE", URL: "https://textures.example/inactive.png", Variant: "CLASSIC"},
			{ID: "active", State: "ACTIVE", URL: "https://textures.example/active.png", Variant: "SLIM"},
		},
		Capes: []microsoft.MinecraftTexture{{ID: "cape", URL: "https://textures.example/cape.png"}},
	}
	if skin := profile.ActiveSkin(); skin == nil || skin.ID != "active" || skin.Variant != "SLIM" {
		t.Fatalf("active skin mismatch: %#v", skin)
	}
	if cape := profile.ActiveCape(); cape == nil || cape.ID != "cape" {
		t.Fatalf("fallback cape mismatch: %#v", cape)
	}
	if texture := (microsoft.MinecraftProfile{}).ActiveSkin(); texture != nil {
		t.Fatalf("empty profile active skin=%#v; want nil", texture)
	}
}

func TestMicrosoftHTTPClientReportsEmptyOwnershipExactly(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"items":[]}`), nil
	})}}
	owned, err := client.CheckGameOwnership(context.Background(), "mc_access")
	if err != nil || owned {
		t.Fatalf("empty entitlement list should mean not owned: owned=%v err=%v", owned, err)
	}
}
