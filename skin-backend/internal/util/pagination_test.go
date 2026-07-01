package util

import (
	"encoding/base64"
	"testing"
)

func TestClampLimit(t *testing.T) {
	cases := []struct {
		name string
		in   any
		def  []int
		want int
	}{
		{name: "nil", in: nil, want: DefaultLimit},
		{name: "negative", in: -1, want: 1},
		{name: "zero", in: 0, want: 1},
		{name: "huge", in: 999999, want: MaxLimit},
		{name: "string", in: "50", want: 50},
		{name: "bad string", in: "abc", want: DefaultLimit},
		{name: "custom default", in: struct{}{}, def: []int{37}, want: 37},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClampLimit(tc.in, tc.def...); got != tc.want {
				t.Fatalf("ClampLimit(%v)=%d want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestCursorRoundTrip(t *testing.T) {
	cursor := EncodeCursor(map[string]any{"last_id": "abc"})
	got, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if got["last_id"] != "abc" {
		t.Fatalf("unexpected cursor payload: %#v", got)
	}
	if empty := EncodeCursor(map[string]any{}); empty != "" {
		t.Fatalf("empty cursor payload should encode to empty string, got %q", empty)
	}
	if decoded, err := DecodeCursor(""); err != nil || decoded != nil {
		t.Fatalf("empty cursor decoded=%#v err=%v; want nil nil", decoded, err)
	}
	if decoded, err := DecodeCursor("not-valid-base64"); err == nil || decoded != nil {
		t.Fatalf("invalid base64 cursor decoded=%#v err=%v; want nil and an error", decoded, err)
	}
	badJSON := base64.RawURLEncoding.EncodeToString([]byte("{not-json"))
	if decoded, err := DecodeCursor(badJSON); err == nil || decoded != nil {
		t.Fatalf("invalid JSON cursor decoded=%#v err=%v; want nil and an error", decoded, err)
	}
	nullCursor := base64.RawURLEncoding.EncodeToString([]byte("null"))
	if decoded, err := DecodeCursor(nullCursor); err == nil || decoded != nil {
		t.Fatalf("non-empty null cursor decoded=%#v err=%v; want nil and an error", decoded, err)
	}
}

func TestCursorInt64AcceptsOnlyExactNonNegativeIntegers(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  int64
	}{
		{value: int64(0), want: 0},
		{value: int(42), want: 42},
		{value: float64(12345), want: 12345},
	} {
		got, ok := CursorInt64(tc.value)
		if !ok || got != tc.want {
			t.Fatalf("CursorInt64(%#v)=(%d,%v); want (%d,true)", tc.value, got, ok, tc.want)
		}
	}
	for _, value := range []any{float64(1.5), float64(-1), float64(1e30), int64(-1), -1, "1", nil} {
		if got, ok := CursorInt64(value); ok {
			t.Fatalf("CursorInt64(%#v)=(%d,true); want rejection", value, got)
		}
	}
}

func TestNormalizePublicURL(t *testing.T) {
	if got := NormalizePublicURL("https://skin.example.com/skin/api/"); got != "https://skin.example.com/skin/api" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
	if got := NormalizePublicURL("skin.example.com/skinapi"); got != "https://skin.example.com/skinapi" {
		t.Fatalf("unexpected host-only URL: %q", got)
	}
	if got := NormalizePublicURL("/skinapi"); got != "" {
		t.Fatalf("relative URL should be rejected, got %q", got)
	}
}

func TestProfileNameHelpers(t *testing.T) {
	for _, name := range []string{"Player1", "a", "A_b_3", "xxxxxxxxxxxxxxxx"} {
		if !ValidProfileName(name) {
			t.Fatalf("expected valid profile name %q", name)
		}
	}
	for _, name := range []string{"", "xxxxxxxxxxxxxxxxx", "has space", "dash-name", "dot.name"} {
		if ValidProfileName(name) {
			t.Fatalf("expected invalid profile name %q", name)
		}
	}

	got, err := GenerateUniqueProfileName("Steve", func(string) bool { return false }, 5)
	if err != nil || got != "Steve" {
		t.Fatalf("available base got=%q err=%v", got, err)
	}
	taken := map[string]bool{"Steve": true, "Steve_1": true, "Steve_2": true}
	got, err = GenerateUniqueProfileName("Steve", func(n string) bool { return taken[n] }, 5)
	if err != nil || got != "Steve_3" {
		t.Fatalf("suffix got=%q err=%v", got, err)
	}
	if _, err := GenerateUniqueProfileName("Steve", func(string) bool { return true }, 5); err == nil {
		t.Fatal("expected exhaustion error")
	}
}
