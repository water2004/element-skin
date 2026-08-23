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

	"element-skin/backend/internal/util"
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
		return ProviderMetadata{}, util.ClassifiedError{Object: "identity_provider", Operation: "discover", Reason: "unavailable", Cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return ProviderMetadata{}, util.ClassifiedError{Object: "identity_provider", Operation: "discover", Reason: "denied", Cause: fmt.Errorf("status %d", resp.StatusCode)}
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	var metadata ProviderMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return ProviderMetadata{}, util.ClassifiedError{Object: "identity_provider", Operation: "discover", Reason: "invalid", Cause: err}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProviderMetadata{}, util.ClassifiedError{Object: "identity_provider", Operation: "discover", Reason: "invalid", Cause: errors.New("multiple JSON values")}
	}
	return metadata, nil
}
