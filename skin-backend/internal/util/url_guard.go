package util

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var ErrUnsafeURL = errors.New("unsafe outbound URL")

func ValidateOutboundURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrUnsafeURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrUnsafeURL
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return ErrUnsafeURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return ErrUnsafeURL
		}
		return nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return ErrUnsafeURL
	}
	for _, ip := range addrs {
		if !isPublicIP(ip) {
			return ErrUnsafeURL
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified())
}
