package microsoft_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

type fakeMicrosoftClient struct {
	calls   []string
	failAt  string
	hasGame bool
	profile *microsoft.MinecraftProfile
}

func (f *fakeMicrosoftClient) stage(name string) error {
	f.calls = append(f.calls, name)
	if f.failAt == name {
		return errors.New(name + " failed")
	}
	return nil
}

func (f *fakeMicrosoftClient) AuthenticateXBL(context.Context, string) (string, string, error) {
	if err := f.stage("xbl"); err != nil {
		return "", "", err
	}
	return "xbl_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateXSTS(context.Context, string) (string, string, error) {
	if err := f.stage("xsts"); err != nil {
		return "", "", err
	}
	return "xsts_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateMinecraft(context.Context, string, string) (string, error) {
	if err := f.stage("minecraft"); err != nil {
		return "", err
	}
	return "mc_access_token", nil
}

func (f *fakeMicrosoftClient) CheckGameOwnership(context.Context, string) (bool, error) {
	if err := f.stage("ownership"); err != nil {
		return false, err
	}
	return f.hasGame, nil
}

func (f *fakeMicrosoftClient) GetMinecraftProfile(context.Context, string) (*microsoft.MinecraftProfile, error) {
	if err := f.stage("profile"); err != nil {
		return nil, err
	}
	return f.profile, nil
}

func TestMicrosoftProfileFlowResolvesOwnedProfileExactly(t *testing.T) {
	client := &fakeMicrosoftClient{hasGame: true, profile: &microsoft.MinecraftProfile{ID: "uuid", Name: "McPlayer"}}
	result, err := (microsoft.ProfileFlow{Client: client}).Resolve(context.Background(), "ms_access_token")
	if err != nil || !result.HasGame || result.Profile == nil || result.Profile.ID != "uuid" || result.Profile.Name != "McPlayer" {
		t.Fatalf("unexpected profile flow result=%#v err=%v", result, err)
	}
	want := []string{"xbl", "xsts", "minecraft", "ownership", "profile"}
	if strings.Join(client.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected call order: %#v", client.calls)
	}
}

func TestMicrosoftProfileFlowStopsWithoutProfileWhenGameIsNotOwned(t *testing.T) {
	client := &fakeMicrosoftClient{}
	result, err := (microsoft.ProfileFlow{Client: client}).Resolve(context.Background(), "ms_access_token")
	if err != nil || result.HasGame || result.Profile != nil {
		t.Fatalf("unowned result=%#v err=%v; want empty non-error result", result, err)
	}
	want := []string{"xbl", "xsts", "minecraft", "ownership"}
	if strings.Join(client.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unowned call order=%#v; want %#v", client.calls, want)
	}
}

func TestMicrosoftProfileFlowStopsExactlyAtEachFailedStage(t *testing.T) {
	stages := []string{"xbl", "xsts", "minecraft", "ownership", "profile"}
	for failedIndex, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			client := &fakeMicrosoftClient{failAt: stage, hasGame: true}
			result, err := (microsoft.ProfileFlow{Client: client}).Resolve(context.Background(), "ms_access_token")
			if result != (microsoft.ProfileResult{}) || err == nil || err.Error() != stage+" failed" {
				t.Fatalf("failed stage %q result=%#v err=%v; want empty result and exact error", stage, result, err)
			}
			wantCalls := stages[:failedIndex+1]
			if strings.Join(client.calls, ",") != strings.Join(wantCalls, ",") {
				t.Fatalf("failed stage %q calls=%#v; want exact prefix %#v", stage, client.calls, wantCalls)
			}
		})
	}
}
