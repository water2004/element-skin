package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestFallbackStatusHTTPSurfacesProbeHistory(t *testing.T) {
	db, h, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()

	svc := settings.Settings{DB: db, Redis: redis}
	if err := svc.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{
			map[string]any{
				"priority":     1,
				"session_url":  "https://session.test",
				"account_url":  "https://account.test",
				"services_url": "https://services.test",
				"note":         "primary",
			},
			map[string]any{
				"priority":     2,
				"session_url":  "https://session2.test",
				"account_url":  "https://account2.test",
				"services_url": "https://services2.test",
				"note":         "secondary",
			},
		},
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil || len(endpoints) != 2 {
		t.Fatalf("seed endpoints: %v", err)
	}
	primaryID, _ := endpoints[0]["id"].(int)
	secondaryID, _ := endpoints[1]["id"].(int)

	now := time.Now()
	if err := redis.AppendProbeSamples(ctx, []redisstore.ProbeSample{
		{EndpointID: primaryID, CheckedAt: now.Add(-2 * time.Hour).UnixMilli(), Session: "up", Account: "up", Services: "up"},
		{EndpointID: primaryID, CheckedAt: now.UnixMilli(), Session: "up", Account: "down", Services: "up"},
		{EndpointID: secondaryID, CheckedAt: now.UnixMilli(), Session: "down", Account: "down", Services: "down"},
	}, redisstore.ProbeHistoryRetention); err != nil {
		t.Fatalf("append samples: %v", err)
	}

	rec := doJSON(t, h, http.MethodGet, "/v1/public/fallback-status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Endpoints []struct {
			ID     int    `json:"id"`
			Note   string `json:"note"`
			Latest *struct {
				Session  string `json:"session"`
				Account  string `json:"account"`
				Services string `json:"services"`
			} `json:"latest"`
			History []struct {
				Session string `json:"session"`
			} `json:"history"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(body.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d body=%s", len(body.Endpoints), rec.Body.String())
	}

	primary := findEndpointByID(body.Endpoints, primaryID)
	if primary == nil || len(primary.History) != 2 {
		t.Fatalf("primary should have two history entries: %#v body=%s", primary, rec.Body.String())
	}
	if primary.Latest == nil || primary.Latest.Account != "down" || primary.Latest.Session != "up" {
		t.Fatalf("primary latest should reflect newest sample: %#v", primary.Latest)
	}
	secondary := findEndpointByID(body.Endpoints, secondaryID)
	if secondary == nil || secondary.Latest == nil || secondary.Latest.Session != "down" {
		t.Fatalf("secondary latest mismatch: %#v", secondary)
	}
	if !strings.Contains(rec.Body.String(), `"retention_ms"`) || !strings.Contains(rec.Body.String(), `"generated_at"`) {
		t.Fatalf("response should include retention_ms and generated_at: %s", rec.Body.String())
	}
}

func findEndpointByID(list []struct {
	ID     int    `json:"id"`
	Note   string `json:"note"`
	Latest *struct {
		Session  string `json:"session"`
		Account  string `json:"account"`
		Services string `json:"services"`
	} `json:"latest"`
	History []struct {
		Session string `json:"session"`
	} `json:"history"`
}, id int) *struct {
	ID     int    `json:"id"`
	Note   string `json:"note"`
	Latest *struct {
		Session  string `json:"session"`
		Account  string `json:"account"`
		Services string `json:"services"`
	} `json:"latest"`
	History []struct {
		Session string `json:"session"`
	} `json:"history"`
} {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}
