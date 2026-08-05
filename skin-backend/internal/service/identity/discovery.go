package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ProviderMetadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type Discovery interface {
	Discover(context.Context, string) (ProviderMetadata, error)
}

type HTTPDiscovery struct {
	Client *http.Client
}

func (d HTTPDiscovery) Discover(ctx context.Context, issuer string) (ProviderMetadata, error) {
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return ProviderMetadata{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return ProviderMetadata{}, fmt.Errorf("fetch OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return ProviderMetadata{}, fmt.Errorf("fetch OIDC discovery document: status %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	var metadata ProviderMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return ProviderMetadata{}, fmt.Errorf("decode OIDC discovery document: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProviderMetadata{}, errors.New("decode OIDC discovery document: multiple JSON values")
	}
	return metadata, nil
}
