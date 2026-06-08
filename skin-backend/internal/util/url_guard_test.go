package util

import "testing"

func TestValidateOutboundURLBlocksUnsafeAndAllowsPublicLiteral(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/x",
		"http://localhost/x",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.5/x",
		"http://192.168.1.1/x",
		"http://172.16.0.1/x",
		"http://[::1]/x",
		"http://0.0.0.0/x",
		"file:///etc/passwd",
		"ftp://internal/x",
		"",
	}
	for _, raw := range blocked {
		if err := ValidateOutboundURL(raw); err == nil {
			t.Fatalf("expected %q to be blocked", raw)
		}
	}
	if err := ValidateOutboundURL("http://1.1.1.1/x"); err != nil {
		t.Fatalf("public IP literal should be allowed: %v", err)
	}
}
