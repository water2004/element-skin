package admin

import (
	"encoding/json"
	"errors"
	"testing"

	"element-skin/backend/internal/util"
)

func TestNoticePatchInputParsesEverySupportedFieldExactly(t *testing.T) {
	startsAt := int64(1700000000000)
	raw := map[string]json.RawMessage{
		"type":             json.RawMessage(`"announcement"`),
		"title":            json.RawMessage(`"Title"`),
		"summary":          json.RawMessage(`"Summary"`),
		"content_markdown": json.RawMessage(`"# Body"`),
		"display_mode":     json.RawMessage(`"detail"`),
		"level":            json.RawMessage(`"warning"`),
		"link_text":        json.RawMessage(`"Open"`),
		"link_url":         json.RawMessage(`"/notifications/1"`),
		"audience":         json.RawMessage(`"admins"`),
		"enabled":          json.RawMessage(`true`),
		"pinned":           json.RawMessage(`false`),
		"dismissible":      json.RawMessage(`true`),
		"starts_at":        json.RawMessage(`1700000000000`),
		"ends_at":          json.RawMessage(`null`),
	}
	input, err := noticePatchInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if input.Type == nil || *input.Type != "announcement" ||
		input.Title == nil || *input.Title != "Title" ||
		input.Summary == nil || *input.Summary != "Summary" ||
		input.ContentMarkdown == nil || *input.ContentMarkdown != "# Body" ||
		input.DisplayMode == nil || *input.DisplayMode != "detail" ||
		input.Level == nil || *input.Level != "warning" ||
		input.LinkText == nil || *input.LinkText != "Open" ||
		input.LinkURL == nil || *input.LinkURL != "/notifications/1" ||
		input.Audience == nil || *input.Audience != "admins" {
		t.Fatalf("string patch fields mismatch: %#v", input)
	}
	if input.Enabled == nil || *input.Enabled != true ||
		input.Pinned == nil || *input.Pinned != false ||
		input.Dismissible == nil || *input.Dismissible != true {
		t.Fatalf("bool patch fields mismatch: %#v", input)
	}
	if input.StartsAt == nil || *input.StartsAt != startsAt || input.ClearStartsAt ||
		input.EndsAt != nil || !input.ClearEndsAt {
		t.Fatalf("time patch fields mismatch: %#v", input)
	}
}

func TestNoticePatchInputRejectsInvalidValueTypesExactly(t *testing.T) {
	cases := []struct {
		name string
		raw  map[string]json.RawMessage
	}{
		{name: "string", raw: map[string]json.RawMessage{"title": json.RawMessage(`123`)}},
		{name: "bool", raw: map[string]json.RawMessage{"enabled": json.RawMessage(`"yes"`)}},
		{name: "int", raw: map[string]json.RawMessage{"starts_at": json.RawMessage(`"soon"`)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := noticePatchInput(tc.raw)
			var httpErr util.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != 400 || httpErr.Error() != "patch_value.decode.invalid" {
				t.Fatalf("patch %s error mismatch: %#v", tc.name, err)
			}
		})
	}
}
