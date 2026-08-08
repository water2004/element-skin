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
	dispatchBatchSize   = 200
	deliveryBatchSize   = 50
	deliveryConcurrency = 10
	deliveryLease       = 30 * time.Second
	requestTimeout      = 10 * time.Second
	maxAttempts         = 12
	maxDeliveryAge      = 72 * time.Hour
	terminalRetention   = 7 * 24 * time.Hour
	cleanupBatchSize    = 5000
	maxResponseBytes    = 64 << 10
)

type Worker struct {
	DB         *database.DB
	Config     config.Config
	HTTPClient *http.Client
	Now        func() time.Time
}

func NewWorker(db *database.DB, cfg config.Config) Worker {
	return Worker{
		DB:         db,
		Config:     cfg,
		HTTPClient: newSafeHTTPClient(),
		Now:        time.Now,
	}
}

func (w Worker) Run(ctx context.Context) {
	workTicker := time.NewTicker(500 * time.Millisecond)
	cleanupTicker := time.NewTicker(15 * time.Minute)
	defer workTicker.Stop()
	defer cleanupTicker.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("webhook worker batch failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-workTicker.C:
		case <-cleanupTicker.C:
			cutoff := w.nowMS() - int64(terminalRetention/time.Millisecond)
			if _, err := w.DB.Webhooks.DeleteTerminalEventsBefore(ctx, cutoff, cleanupBatchSize); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("webhook worker cleanup failed: %v", err)
			}
		}
	}
}

func (w Worker) RunOnce(ctx context.Context) error {
	if err := w.DispatchBatch(ctx); err != nil {
		return err
	}
	return w.DeliverBatch(ctx)
}

func (w Worker) DispatchBatch(ctx context.Context) error {
	events, err := w.DB.Webhooks.ListPendingEvents(ctx, dispatchBatchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		definition, ok := corewebhook.DefinitionByType(event.Type)
		if !ok {
			if err := w.DB.Webhooks.CompleteExpansion(ctx, event.ID, nil, w.nowMS()); err != nil {
				return err
			}
			continue
		}
		targetClientID := ""
		if definition.TargetClient {
			targetClientID = event.TargetClientID
		}
		endpoints, err := w.DB.Webhooks.ListSubscribedEndpoints(ctx, event.Type, targetClientID)
		if err != nil {
			return err
		}
		endpointIDs := make([]string, 0, len(endpoints))
		for _, endpoint := range endpoints {
			allowed, err := w.endpointAuthorized(ctx, event, endpoint, definition)
			if err != nil {
				return err
			}
			if allowed {
				endpointIDs = append(endpointIDs, endpoint.ID)
			}
		}
		if err := w.DB.Webhooks.CompleteExpansion(ctx, event.ID, endpointIDs, w.nowMS()); err != nil {
			return err
		}
	}
	return nil
}

func (w Worker) DeliverBatch(ctx context.Context) error {
	now := w.nowMS()
	deliveries, err := w.DB.Webhooks.ClaimDueDeliveries(ctx, now, now+int64(deliveryLease/time.Millisecond), deliveryBatchSize)
	if err != nil {
		return err
	}
	semaphore := make(chan struct{}, deliveryConcurrency)
	errorsByDelivery := make(chan error, len(deliveries))
	var wait sync.WaitGroup
	for _, delivery := range deliveries {
		delivery := delivery
		wait.Add(1)
		semaphore <- struct{}{}
		go func() {
			defer wait.Done()
			defer func() { <-semaphore }()
			if err := w.deliver(ctx, delivery); err != nil {
				errorsByDelivery <- err
			}
		}()
	}
	wait.Wait()
	close(errorsByDelivery)
	for err := range errorsByDelivery {
		return err
	}
	return nil
}

func (w Worker) deliver(ctx context.Context, delivery model.WebhookDelivery) error {
	now := w.nowMS()
	definition, ok := corewebhook.DefinitionByType(delivery.Event.Type)
	if !ok || delivery.Endpoint.Status != "active" {
		return w.markDead(ctx, delivery, now, nil, "webhook endpoint or event is inactive")
	}
	allowed, err := w.endpointAuthorized(ctx, delivery.Event, delivery.Endpoint, definition)
	if err != nil {
		return err
	}
	if !allowed {
		return w.markDead(ctx, delivery, now, nil, "permission no longer allows webhook event")
	}
	body, err := json.Marshal(map[string]any{
		"id":         delivery.Event.ID,
		"type":       delivery.Event.Type,
		"created_at": delivery.Event.CreatedAt,
		"data":       delivery.Event.Data,
	})
	if err != nil {
		return w.markDead(ctx, delivery, now, nil, "encode webhook event")
	}
	box, err := util.NewSecretBox(w.Config.IdentityEncryptionKey)
	if err != nil {
		return err
	}
	secret, err := box.Decrypt(delivery.Endpoint.SecretCiphertext)
	if err != nil {
		return w.markDead(ctx, delivery, now, nil, "decrypt webhook signing secret")
	}
	timestamp := strconv.FormatInt(now, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.Endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return w.markDead(ctx, delivery, now, nil, "create webhook request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Element-Skin-Webhook/1")
	req.Header.Set("Webhook-Id", delivery.Event.ID)
	req.Header.Set("Webhook-Delivery", delivery.ID)
	req.Header.Set("Webhook-Timestamp", timestamp)
	req.Header.Set("Webhook-Signature", sign(secret, timestamp, body))
	res, requestErr := w.httpClient().Do(req)
	if requestErr != nil {
		return w.retryOrDead(ctx, delivery, now, nil, requestErr.Error())
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseBytes))
	_ = res.Body.Close()
	if res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices {
		updated, err := w.DB.Webhooks.CompleteDelivery(ctx, delivery.ID, now, res.StatusCode)
		if err != nil {
			return err
		}
		if !updated {
			return fmt.Errorf("webhook delivery %s is no longer processing", delivery.ID)
		}
		return nil
	}
	return w.retryOrDead(ctx, delivery, now, &res.StatusCode, fmt.Sprintf("webhook endpoint returned HTTP %d", res.StatusCode))
}

func (w Worker) endpointAuthorized(ctx context.Context, event model.WebhookEvent, endpoint model.WebhookEndpoint, definition corewebhook.Definition) (bool, error) {
	clientPermissionIDs, err := w.DB.OAuth.ClientPermissionIDs(ctx, endpoint.ClientID)
	if err != nil {
		return false, err
	}
	clientAllowed := make(map[int64]bool, len(clientPermissionIDs))
	for _, permissionID := range clientPermissionIDs {
		clientAllowed[permissionID] = true
	}
	if definition.TargetClient {
		if event.TargetClientID != endpoint.ClientID {
			return false, nil
		}
		owned := permission.MustDefinitionByCode(definition.OwnedPermissionCode)
		return clientAllowed[int64(owned.ID)], nil
	}
	if definition.ApplicationPermissionCode != "" {
		applicationPermission := permission.MustDefinitionByCode(definition.ApplicationPermissionCode)
		if clientAllowed[int64(applicationPermission.ID)] {
			actor, err := w.DB.Permissions.ActorForClient(ctx, endpoint.ClientID, permissiondb.EffectiveOptions{
				SessionKind: permission.SessionKindClient,
				Entrypoint:  permission.EntrypointAPI,
			})
			if err != nil {
				return false, err
			}
			if actor.Has(applicationPermission) {
				return true, nil
			}
		}
	}
	if event.SubjectUserID == "" || definition.OwnedPermissionCode == "" {
		return false, nil
	}
	grant, err := w.DB.OAuth.ActiveGrantByUserClient(ctx, event.SubjectUserID, endpoint.ClientID)
	if err != nil || grant == nil {
		return false, err
	}
	actor, err := w.DB.Permissions.ActorForUser(ctx, event.SubjectUserID, permissiondb.EffectiveOptions{
		SessionKind:       permission.SessionKindDelegated,
		Entrypoint:        permission.EntrypointDashboard,
		DelegatedGrantID:  grant.ID,
		DelegatedClientID: endpoint.ClientID,
	})
	if err != nil {
		return false, err
	}
	return actor.Has(permission.MustDefinitionByCode(definition.OwnedPermissionCode)), nil
}

func (w Worker) retryOrDead(ctx context.Context, delivery model.WebhookDelivery, now int64, httpStatus *int, detail string) error {
	age := time.Duration(now-delivery.CreatedAt) * time.Millisecond
	if delivery.AttemptCount >= maxAttempts || age >= maxDeliveryAge {
		return w.markDead(ctx, delivery, now, httpStatus, detail)
	}
	nextAttemptAt := now + int64(retryDelay(delivery.ID, delivery.AttemptCount)/time.Millisecond)
	updated, err := w.DB.Webhooks.FailDelivery(ctx, delivery.ID, "pending", nextAttemptAt, now, httpStatus, truncateDetail(detail))
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("webhook delivery %s is no longer processing", delivery.ID)
	}
	return nil
}

func (w Worker) markDead(ctx context.Context, delivery model.WebhookDelivery, now int64, httpStatus *int, detail string) error {
	updated, err := w.DB.Webhooks.FailDelivery(ctx, delivery.ID, "dead", now, now, httpStatus, truncateDetail(detail))
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("webhook delivery %s is no longer processing", delivery.ID)
	}
	return nil
}

func (w Worker) nowMS() int64 {
	if w.Now == nil {
		return time.Now().UnixMilli()
	}
	return w.Now().UnixMilli()
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
			if !publicIP(address.IP) {
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

func publicIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}
