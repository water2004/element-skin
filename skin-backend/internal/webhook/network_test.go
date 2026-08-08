package webhook_test

import (
	"net"
	"testing"

	"element-skin/backend/internal/webhook"
)

func TestPublicIPClassificationMatchesWebhookNetworkPolicyExactly(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "public ipv4", raw: "8.8.8.8", want: true},
		{name: "public ipv6", raw: "2606:4700:4700::1111", want: true},
		{name: "private ipv4", raw: "10.0.0.1", want: false},
		{name: "private ipv6", raw: "fd00::1", want: false},
		{name: "loopback ipv4", raw: "127.0.0.1", want: false},
		{name: "loopback ipv6", raw: "::1", want: false},
		{name: "link local", raw: "169.254.1.1", want: false},
		{name: "multicast", raw: "224.0.0.1", want: false},
		{name: "unspecified", raw: "0.0.0.0", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ip := net.ParseIP(test.raw)
			if got := webhook.IsPublicIP(ip); got != test.want {
				t.Fatalf("IsPublicIP(%q)=%v want=%v", test.raw, got, test.want)
			}
		})
	}
	if webhook.IsPublicIP(nil) {
		t.Fatal("IsPublicIP(nil)=true want=false")
	}
}
