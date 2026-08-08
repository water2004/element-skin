package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
	corewebhook "element-skin/backend/internal/webhook"
)

const (
	dispatchBatchSize     = 200
	deliveryBatchSize     = 50
	deliveryConcurrency   = 10
	deliveryBatches       = 4
	expansionLease        = 30 * time.Second
	deliveryLease         = 30 * time.Second
	defaultPollInterval   = 500 * time.Millisecond
	defaultActiveInterval = 3 * time.Second
	requestTimeout        = 10 * time.Second
	maxAttempts           = 12
	maxDeliveryAge        = 72 * time.Hour
	terminalRetention     = 7 * 24 * time.Hour
	cleanupBatchSize      = 5000
	maxResponseBytes      = 64 << 10
)

type Worker struct {
	DB             *database.DB
	Config         config.Config
	HTTPClient     *http.Client
	Now            func() time.Time
	PollInterval   time.Duration
	ActiveInterval time.Duration
}

type BatchResult struct {
	ExpandedEvents      int
	ProcessedDeliveries int
}

func (r BatchResult) Worked() bool {
	return r.ExpandedEvents > 0 || r.ProcessedDeliveries > 0
}

func NewWorker(db *database.DB, cfg config.Config) Worker {
	worker := Worker{
		DB:         db,
		Config:     cfg,
		HTTPClient: newSafeHTTPClient(),
		Now:        time.Now,
	}
	if cfg.WebhookWorkerActiveIntervalMS > 0 {
		worker.ActiveInterval = time.Duration(cfg.WebhookWorkerActiveIntervalMS) * time.Millisecond
	}
	return worker
}

func (w Worker) Run(ctx context.Context) {
	workTicker := time.NewTicker(w.pollInterval())
	cleanupTicker := time.NewTicker(15 * time.Minute)
	defer workTicker.Stop()
	defer cleanupTicker.Stop()
	for {
		result, err := w.RunOnce(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("webhook worker batch failed: %v", err)
		}
		if err == nil && result.Worked() {
			if !w.waitAfterWork(ctx, cleanupTicker.C) {
				return
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-workTicker.C:
		case <-cleanupTicker.C:
			w.cleanup(ctx)
		}
	}
}

func (w Worker) RunOnce(ctx context.Context) (BatchResult, error) {
	expanded, err := w.DispatchBatch(ctx)
	result := BatchResult{ExpandedEvents: expanded}
	if err != nil {
		return result, err
	}
	for range deliveryBatches {
		processed, err := w.DeliverBatch(ctx)
		result.ProcessedDeliveries += processed
		if err != nil {
			return result, err
		}
		if processed < deliveryBatchSize {
			break
		}
	}
	return result, nil
}

func (w Worker) DispatchBatch(ctx context.Context) (int, error) {
	now := w.nowMS()
	events, err := w.DB.Webhooks.ClaimPendingEvents(
		ctx,
		now,
		now+int64(expansionLease/time.Millisecond),
		dispatchBatchSize,
	)
	if err != nil {
		return 0, err
	}
	subscriptions := make(map[subscriptionCacheKey][]model.WebhookEndpoint)
	authorizations := make(map[authorizationCacheKey]bool)
	expansions := make([]model.WebhookExpansion, 0, len(events))
	for _, event := range events {
		expansion := model.WebhookExpansion{EventID: event.ID, LeaseToken: event.ExpansionLeaseToken}
		definition, ok := corewebhook.DefinitionByType(event.Type)
		if !ok {
			expansions = append(expansions, expansion)
			continue
		}
		targetClientID := ""
		if definition.TargetClient {
			targetClientID = event.TargetClientID
		}
		subscriptionKey := subscriptionCacheKey{EventType: event.Type, TargetClientID: targetClientID}
		endpoints, cached := subscriptions[subscriptionKey]
		if !cached {
			endpoints, err = w.DB.Webhooks.ListSubscribedEndpoints(ctx, event.Type, targetClientID)
			if err != nil {
				return 0, err
			}
			subscriptions[subscriptionKey] = endpoints
		}
		endpointIDs := make([]string, 0, len(endpoints))
		for _, endpoint := range endpoints {
			authorizationKey := authorizationCacheKey{
				EventType:        event.Type,
				TargetClientID:   event.TargetClientID,
				SubjectUserID:    event.SubjectUserID,
				EndpointClientID: endpoint.ClientID,
			}
			allowed, cached := authorizations[authorizationKey]
			if !cached {
				allowed, err = w.endpointAuthorized(ctx, event, endpoint, definition)
				if err != nil {
					return 0, err
				}
				authorizations[authorizationKey] = allowed
			}
			if allowed {
				endpointIDs = append(endpointIDs, endpoint.ID)
			}
		}
		expansion.EndpointIDs = endpointIDs
		expansions = append(expansions, expansion)
	}
	completed, _, err := w.DB.Webhooks.CompleteExpansions(ctx, expansions, w.nowMS())
	return int(completed), err
}

func (w Worker) DeliverBatch(ctx context.Context) (int, error) {
	now := w.nowMS()
	deliveries, err := w.DB.Webhooks.ClaimDueDeliveries(ctx, now, now+int64(deliveryLease/time.Millisecond), deliveryBatchSize)
	if err != nil {
		return 0, err
	}
	semaphore := make(chan struct{}, deliveryConcurrency)
	errorsByDelivery := make(chan error, len(deliveries))
	outcomesByDelivery := make(chan model.WebhookDeliveryOutcome, len(deliveries))
	authorizations := newBatchAuthorizationCache()
	var wait sync.WaitGroup
	for _, delivery := range deliveries {
		delivery := delivery
		wait.Add(1)
		semaphore <- struct{}{}
		go func() {
			defer wait.Done()
			defer func() { <-semaphore }()
			outcome, err := w.deliver(ctx, delivery, authorizations)
			if err != nil {
				errorsByDelivery <- err
				return
			}
			outcomesByDelivery <- outcome
		}()
	}
	wait.Wait()
	close(errorsByDelivery)
	close(outcomesByDelivery)
	outcomes := make([]model.WebhookDeliveryOutcome, 0, len(deliveries))
	for outcome := range outcomesByDelivery {
		outcomes = append(outcomes, outcome)
	}
	updated, err := w.DB.Webhooks.CompleteDeliveryOutcomes(ctx, outcomes)
	if err != nil {
		return len(deliveries), err
	}
	if updated != int64(len(outcomes)) {
		return len(deliveries), fmt.Errorf("completed %d of %d webhook delivery outcomes", updated, len(outcomes))
	}
	for err := range errorsByDelivery {
		return len(deliveries), err
	}
	return len(deliveries), nil
}

type subscriptionCacheKey struct {
	EventType      string
	TargetClientID string
}

type authorizationCacheKey struct {
	EventType        string
	TargetClientID   string
	SubjectUserID    string
	EndpointClientID string
}

type batchAuthorizationCache struct {
	mu      sync.Mutex
	entries map[authorizationCacheKey]*batchAuthorizationEntry
}

type batchAuthorizationEntry struct {
	ready   chan struct{}
	allowed bool
	err     error
}

func newBatchAuthorizationCache() *batchAuthorizationCache {
	return &batchAuthorizationCache{entries: make(map[authorizationCacheKey]*batchAuthorizationEntry)}
}

func (c *batchAuthorizationCache) resolve(ctx context.Context, key authorizationCacheKey, load func() (bool, error)) (bool, error) {
	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &batchAuthorizationEntry{ready: make(chan struct{})}
		c.entries[key] = entry
	}
	c.mu.Unlock()
	if ok {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-entry.ready:
			return entry.allowed, entry.err
		}
	}
	entry.allowed, entry.err = load()
	close(entry.ready)
	return entry.allowed, entry.err
}

func (w Worker) deliver(ctx context.Context, delivery model.WebhookDelivery, authorizations *batchAuthorizationCache) (model.WebhookDeliveryOutcome, error) {
	now := w.nowMS()
	definition, ok := corewebhook.DefinitionByType(delivery.Event.Type)
	if !ok || delivery.Endpoint.Status != "active" {
		return markDead(delivery, now, nil, "webhook endpoint or event is inactive"), nil
	}
	authorizationKey := authorizationCacheKey{
		EventType:        delivery.Event.Type,
		TargetClientID:   delivery.Event.TargetClientID,
		SubjectUserID:    delivery.Event.SubjectUserID,
		EndpointClientID: delivery.Endpoint.ClientID,
	}
	allowed, err := authorizations.resolve(ctx, authorizationKey, func() (bool, error) {
		return w.endpointAuthorized(ctx, delivery.Event, delivery.Endpoint, definition)
	})
	if err != nil {
		return model.WebhookDeliveryOutcome{}, err
	}
	if !allowed {
		return markDead(delivery, now, nil, "permission no longer allows webhook event"), nil
	}
	body, err := json.Marshal(map[string]any{
		"id":         delivery.Event.ID,
		"type":       delivery.Event.Type,
		"created_at": delivery.Event.CreatedAt,
		"data":       delivery.Event.Data,
	})
	if err != nil {
		return markDead(delivery, now, nil, "encode webhook event"), nil
	}
	box, err := util.NewSecretBox(w.Config.IdentityEncryptionKey)
	if err != nil {
		return model.WebhookDeliveryOutcome{}, err
	}
	secret, err := box.Decrypt(delivery.Endpoint.SecretCiphertext)
	if err != nil {
		return markDead(delivery, now, nil, "decrypt webhook signing secret"), nil
	}
	timestamp := strconv.FormatInt(now, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.Endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return markDead(delivery, now, nil, "create webhook request"), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Element-Skin-Webhook/1")
	req.Header.Set("Webhook-Id", delivery.Event.ID)
	req.Header.Set("Webhook-Delivery", delivery.ID)
	req.Header.Set("Webhook-Timestamp", timestamp)
	req.Header.Set("Webhook-Signature", sign(secret, timestamp, body))
	res, requestErr := w.httpClient().Do(req)
	if requestErr != nil {
		return retryOrDead(delivery, now, nil, requestErr.Error()), nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseBytes))
	_ = res.Body.Close()
	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices {
		httpStatus := res.StatusCode
		return model.WebhookDeliveryOutcome{
			DeliveryID:  delivery.ID,
			LeaseToken:  delivery.LeaseToken,
			Status:      "succeeded",
			UpdatedAt:   now,
			HTTPStatus:  &httpStatus,
			DeliveredAt: &now,
		}, nil
	}
	httpStatus := res.StatusCode
	return retryOrDead(delivery, now, &httpStatus, fmt.Sprintf("webhook endpoint returned HTTP %d", res.StatusCode)), nil
}

func (w Worker) endpointAuthorized(ctx context.Context, event model.WebhookEvent, endpoint model.WebhookEndpoint, definition corewebhook.Definition) (bool, error) {
	if definition.TargetClient {
		if event.TargetClientID != endpoint.ClientID {
			return false, nil
		}
		owned := permission.MustDefinitionByCode(definition.OwnedPermissionCode)
		return w.DB.OAuth.ClientHasPermission(ctx, endpoint.ClientID, int64(owned.ID))
	}
	if definition.ApplicationPermissionCode != "" {
		applicationPermission := permission.MustDefinitionByCode(definition.ApplicationPermissionCode)
		ownedPermission := permission.MustDefinitionByCode(definition.OwnedPermissionCode)
		state, err := w.DB.OAuth.AuthorizationPermissionState(
			ctx,
			event.SubjectUserID,
			endpoint.ClientID,
			int64(ownedPermission.ID),
			int64(applicationPermission.ID),
		)
		if err != nil {
			return false, err
		}
		if state.ApplicationRequested {
			permissions, err := w.DB.Permissions.EffectivePermissionsForSubject(
				ctx,
				permissiondb.SubjectIDForClient(endpoint.ClientID),
				permissiondb.EffectiveOptions{
					SessionKind: permission.SessionKindClient,
					Entrypoint:  permission.EntrypointAPI,
				},
			)
			if err != nil {
				return false, err
			}
			if permissions.Has(applicationPermission.BitIndex) {
				return true, nil
			}
		}
		if event.SubjectUserID == "" || !state.OwnedGranted {
			return false, nil
		}
		permissions, err := w.DB.Permissions.EffectivePermissionsForSubject(
			ctx,
			permissiondb.SubjectIDForUser(event.SubjectUserID),
			permissiondb.EffectiveOptions{
				SessionKind: permission.SessionKindDelegated,
				Entrypoint:  permission.EntrypointDashboard,
			},
		)
		if err != nil {
			return false, err
		}
		return permissions.Has(ownedPermission.BitIndex), nil
	}
	return false, nil
}

func retryOrDead(delivery model.WebhookDelivery, now int64, httpStatus *int, detail string) model.WebhookDeliveryOutcome {
	age := time.Duration(now-delivery.CreatedAt) * time.Millisecond
	if delivery.AttemptCount >= maxAttempts || age >= maxDeliveryAge {
		return markDead(delivery, now, httpStatus, detail)
	}
	return model.WebhookDeliveryOutcome{
		DeliveryID:    delivery.ID,
		LeaseToken:    delivery.LeaseToken,
		Status:        "pending",
		NextAttemptAt: now + int64(retryDelay(delivery.ID, delivery.AttemptCount)/time.Millisecond),
		UpdatedAt:     now,
		HTTPStatus:    httpStatus,
		Detail:        truncateDetail(detail),
	}
}

func markDead(delivery model.WebhookDelivery, now int64, httpStatus *int, detail string) model.WebhookDeliveryOutcome {
	return model.WebhookDeliveryOutcome{
		DeliveryID:    delivery.ID,
		LeaseToken:    delivery.LeaseToken,
		Status:        "dead",
		NextAttemptAt: now,
		UpdatedAt:     now,
		HTTPStatus:    httpStatus,
		Detail:        truncateDetail(detail),
	}
}

func (w Worker) nowMS() int64 {
	if w.Now == nil {
		return time.Now().UnixMilli()
	}
	return w.Now().UnixMilli()
}

func (w Worker) pollInterval() time.Duration {
	if w.PollInterval > 0 {
		return w.PollInterval
	}
	return defaultPollInterval
}

func (w Worker) ActiveWorkInterval() time.Duration {
	if w.ActiveInterval > 0 {
		return w.ActiveInterval
	}
	return defaultActiveInterval
}

func (w Worker) waitAfterWork(ctx context.Context, cleanup <-chan time.Time) bool {
	timer := time.NewTimer(w.ActiveWorkInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-cleanup:
			w.cleanup(ctx)
		case <-timer.C:
			return true
		}
	}
}

func (w Worker) cleanup(ctx context.Context) {
	cutoff := w.nowMS() - int64(terminalRetention/time.Millisecond)
	if _, err := w.DB.Webhooks.DeleteTerminalEventsBefore(ctx, cutoff, cleanupBatchSize); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("webhook worker cleanup failed: %v", err)
	}
}

func (w Worker) httpClient() *http.Client {
	if w.HTTPClient != nil {
		return w.HTTPClient
	}
	return newSafeHTTPClient()
}

func retryDelay(deliveryID string, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > 10 {
		shift = 10
	}
	base := 30 * time.Second * time.Duration(1<<shift)
	if base > 6*time.Hour {
		base = 6 * time.Hour
	}
	sum := sha256.Sum256([]byte(deliveryID + ":" + strconv.Itoa(attempt)))
	jitter := time.Duration(int64(base) / 5 * int64(sum[0]) / 255)
	return base + jitter
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

func truncateDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if len(detail) > 500 {
		return detail[:500]
	}
	return detail
}

func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if !corewebhook.IsPublicIP(address.IP) {
				continue
			}
			connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
			if err == nil {
				return connection, nil
			}
		}
		return nil, errors.New("webhook endpoint did not resolve to a public address")
	}
	return &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("webhook redirects are not followed")
		},
	}
}
