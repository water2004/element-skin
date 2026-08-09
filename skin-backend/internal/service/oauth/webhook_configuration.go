package oauth

import (
	"context"
	"sort"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
	corewebhook "element-skin/backend/internal/webhook"
)

const maxWebhookEndpointsPerClient = 5

func (s Service) prepareWebhookEndpoints(ctx context.Context, clientID, clientType string, inputs []WebhookEndpointInput, permissionCodes []string, updatedAt int64) ([]model.WebhookEndpoint, map[string]string, error) {
	if len(inputs) > maxWebhookEndpointsPerClient {
		return nil, nil, badRequest("too many webhook endpoints")
	}
	existing, err := s.DB.Webhooks.ListEndpointsByClient(ctx, clientID)
	if err != nil {
		return nil, nil, err
	}
	existingByID := make(map[string]model.WebhookEndpoint, len(existing))
	for _, endpoint := range existing {
		existingByID[endpoint.ID] = endpoint
	}
	allowedPermissions := make(map[string]bool, len(permissionCodes))
	for _, code := range permissionCodes {
		allowedPermissions[code] = true
	}
	box, err := util.NewSecretBox(s.Config.IdentityEncryptionKey)
	if err != nil && len(inputs) > 0 {
		return nil, nil, err
	}
	seenIDs := map[string]bool{}
	seenURLs := map[string]bool{}
	endpoints := make([]model.WebhookEndpoint, 0, len(inputs))
	newSecrets := map[string]string{}
	for _, input := range inputs {
		url := strings.TrimSpace(input.URL)
		if !validWebhookURL(url) {
			return nil, nil, badRequest("invalid webhook url")
		}
		if seenURLs[url] {
			return nil, nil, badRequest("duplicate webhook url")
		}
		seenURLs[url] = true
		eventTypes, err := validateWebhookEventTypes(input.EventTypes, clientType, allowedPermissions)
		if err != nil {
			return nil, nil, err
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		status := "active"
		if !enabled {
			status = "disabled"
		}
		endpointID := strings.TrimSpace(input.ID)
		endpoint, exists := existingByID[endpointID]
		if endpointID != "" && !exists {
			return nil, nil, badRequest("invalid webhook endpoint id")
		}
		if seenIDs[endpointID] && endpointID != "" {
			return nil, nil, badRequest("duplicate webhook endpoint id")
		}
		if endpointID == "" {
			randomID, err := util.GenerateUUIDNoDash()
			if err != nil {
				return nil, nil, err
			}
			endpointID = "wh_" + randomID
			rawSecret, _, err := generateToken()
			if err != nil {
				return nil, nil, err
			}
			ciphertext, err := box.Encrypt(rawSecret)
			if err != nil {
				return nil, nil, err
			}
			endpoint = model.WebhookEndpoint{
				ID:               endpointID,
				ClientID:         clientID,
				SecretCiphertext: ciphertext,
				CreatedAt:        updatedAt,
			}
			newSecrets[endpointID] = rawSecret
		}
		seenIDs[endpointID] = true
		endpoint.URL = url
		endpoint.Status = status
		endpoint.EventTypes = eventTypes
		endpoint.UpdatedAt = updatedAt
		endpoints = append(endpoints, endpoint)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].CreatedAt == endpoints[j].CreatedAt {
			return endpoints[i].ID < endpoints[j].ID
		}
		return endpoints[i].CreatedAt < endpoints[j].CreatedAt
	})
	return endpoints, newSecrets, nil
}

func validateWebhookEventTypes(raw []string, clientType string, allowedPermissions map[string]bool) ([]string, error) {
	if len(raw) == 0 {
		return nil, badRequest("webhook events are required")
	}
	seen := map[string]bool{}
	eventTypes := make([]string, 0, len(raw))
	for _, eventType := range raw {
		eventType = strings.TrimSpace(eventType)
		definition, ok := corewebhook.DefinitionByType(eventType)
		if !ok {
			return nil, badRequest("invalid webhook event")
		}
		allowed := definition.DelegatedPermissionCode != "" && allowedPermissions[definition.DelegatedPermissionCode]
		allowed = allowed || clientType == ClientTypeConfidential &&
			definition.ApplicationPermissionCode != "" && allowedPermissions[definition.ApplicationPermissionCode]
		if !allowed {
			return nil, badRequest("webhook event exceeds client permission limit")
		}
		if !seen[eventType] {
			seen[eventType] = true
			eventTypes = append(eventTypes, eventType)
		}
	}
	sort.Strings(eventTypes)
	return eventTypes, nil
}

func webhookEndpointResponse(endpoint model.WebhookEndpoint, secret string) map[string]any {
	out := map[string]any{
		"id":         endpoint.ID,
		"url":        endpoint.URL,
		"status":     endpoint.Status,
		"enabled":    endpoint.Status == "active",
		"events":     append([]string{}, endpoint.EventTypes...),
		"created_at": endpoint.CreatedAt,
		"updated_at": endpoint.UpdatedAt,
	}
	if secret != "" {
		out["signing_secret"] = secret
	}
	return out
}
