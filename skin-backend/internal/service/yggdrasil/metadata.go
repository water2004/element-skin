package yggdrasil

import (
	"context"
	"strings"
)

func (y Yggdrasil) Metadata(ctx context.Context) (map[string]any, error) {
	signer, err := y.signer()
	if err != nil {
		return nil, err
	}
	name, err := y.settings().Get(ctx, "site_name", "皮肤站")
	if err != nil {
		return nil, err
	}
	site := strings.TrimRight(y.Cfg.SiteURL, "/")
	host := strings.TrimPrefix(strings.TrimPrefix(site, "https://"), "http://")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return map[string]any{
		"meta": map[string]any{
			"serverName": name, "implementationName": "element-skin", "implementationVersion": "go",
			"links":                   map[string]any{"homepage": site + "/", "register": site + "/register/"},
			"feature.non_email_login": true,
		},
		"skinDomains":        append(y.Cfg.FallbackDomains, host),
		"signaturePublickey": signer.PublicKeyPEM(),
	}, nil
}
