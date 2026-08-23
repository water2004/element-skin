package oauth

import (
	"context"
	"crypto/subtle"
	"sort"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) authenticateClient(ctx context.Context, clientID, secret string) (*model.OAuthClient, error) {
	client, err := s.DB.OAuth.GetClient(ctx, strings.TrimSpace(clientID))
	if err != nil {
		return nil, err
	}
	if client == nil || client.Status != StatusActive {
		return nil, badRequest("client_id", "verify", "invalid")
	}
	if client.ClientType == ClientTypeConfidential {
		if secret == "" || subtle.ConstantTimeCompare([]byte(client.SecretHash), []byte(util.HashRefreshToken(secret))) != 1 {
			return nil, badRequest("client_secret", "verify", "invalid")
		}
	}
	return client, nil
}

func requestedOrDefaultClientScopes(raw string, actor permission.Actor) ([]string, error) {
	if strings.TrimSpace(raw) != "" {
		codes, err := parseScope(raw)
		if err != nil {
			return nil, err
		}
		for _, code := range codes {
			if !actor.Has(permission.MustDefinitionByCode(code)) {
				return nil, forbidden()
			}
		}
		return codes, nil
	}
	codes := make([]string, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if actor.Has(def) {
			codes = append(codes, def.Code)
		}
	}
	sort.Strings(codes)
	return codes, nil
}
