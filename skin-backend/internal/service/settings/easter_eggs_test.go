package settings

import (
	"testing"

	"element-skin/backend/internal/util"
)

func TestValidateEasterEggsAcceptsTypedListAndDeduplicates(t *testing.T) {
	got, err := ValidateEasterEggs([]string{"christmas", "dragon-boat", "christmas", "mid-autumn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "christmas" || got[1] != "dragon-boat" || got[2] != "mid-autumn" {
		t.Fatalf("typed easter egg list should keep order and remove duplicates: %#v", got)
	}
}

func TestValidateEasterEggsRejectsBadPayloadsExactly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		raw    any
		detail string
	}{
		{name: "not a list", raw: "christmas", detail: "easter_egg_setting.configure.invalid"},
		{name: "unknown id", raw: []any{"christmas", "missing"}, detail: "easter_egg.validate.invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateEasterEggs(tc.raw)
			httpErr, ok := err.(util.HTTPError)
			if !ok {
				t.Fatalf("expected HTTPError, got %T %[1]v", err)
			}
			if httpErr.Status != 400 || httpErr.Error() != tc.detail {
				t.Fatalf("unexpected error: %#v", httpErr)
			}
		})
	}
}
