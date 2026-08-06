package model

const (
	EmailSuffixModeDisabled  = "disabled"
	EmailSuffixModeAllowlist = "allowlist"
	EmailSuffixModeDenylist  = "denylist"
)

type EmailSuffixPolicy struct {
	Mode      string   `json:"mode"`
	Allowlist []string `json:"allowlist"`
	Denylist  []string `json:"denylist"`
}
