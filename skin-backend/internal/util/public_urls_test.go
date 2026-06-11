package util

import "testing"

func TestPublicURLAndProfileNameHelpersExactValues(t *testing.T) {
	if got := NormalizePublicURL("example.com/root/?x=1#frag"); got != "https://example.com/root" {
		t.Fatalf("NormalizePublicURL mismatch: %q", got)
	}
	for _, invalid := range []string{"", "/relative", "http://"} {
		if got := NormalizePublicURL(invalid); got != "" {
			t.Fatalf("NormalizePublicURL(%q)=%q, want empty", invalid, got)
		}
	}
	if !ValidProfileName("Player_123") || ValidProfileName("坏名字") || ValidProfileName("abcdefghijklmnopq") {
		t.Fatal("ValidProfileName exact validation failed")
	}
	exists := map[string]bool{"Role": true, "Role_1": true}
	name, err := GenerateUniqueProfileName("Role", func(s string) bool { return exists[s] }, 3)
	if err != nil || name != "Role_2" {
		t.Fatalf("GenerateUniqueProfileName mismatch: name=%q err=%v", name, err)
	}
	full := "1234567890ABCDEF"
	if got := ProfileNameCandidate(full, 0); got != full {
		t.Fatalf("full-length first candidate = %q; want %q", got, full)
	}
	if got := ProfileNameCandidate(full, 1); got != "1234567890ABCD_1" {
		t.Fatalf("full-length suffixed candidate = %q; want 1234567890ABCD_1", got)
	}
	fullTaken := map[string]bool{full: true, "1234567890ABCD_1": true}
	name, err = GenerateUniqueProfileName(full, func(s string) bool { return fullTaken[s] }, 3)
	if err != nil || name != "1234567890ABCD_2" {
		t.Fatalf("full-length unique name = %q, %v; want 1234567890ABCD_2, nil", name, err)
	}
	if _, err := GenerateUniqueProfileName("Role", func(string) bool { return true }, 2); err == nil {
		t.Fatal("GenerateUniqueProfileName should fail after max attempts")
	}
}
