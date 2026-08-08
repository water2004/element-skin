package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

type Queryer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func ReplaceEndpoints(ctx context.Context, q Queryer, clientID string, endpoints []model.WebhookEndpoint) error {
	endpointIDs := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		endpointIDs = append(endpointIDs, endpoint.ID)
	}
	if _, err := q.Exec(ctx, `
		DELETE FROM webhook_endpoints
		WHERE client_id=$1 AND NOT (id = ANY($2))
	`, clientID, endpointIDs); err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		tag, err := q.Exec(ctx, `
			INSERT INTO webhook_endpoints
				(id, client_id, url, secret_ciphertext, status, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET
				url=EXCLUDED.url,
				secret_ciphertext=EXCLUDED.secret_ciphertext,
				status=EXCLUDED.status,
				updated_at=EXCLUDED.updated_at
			WHERE webhook_endpoints.client_id=EXCLUDED.client_id
		`, endpoint.ID, clientID, endpoint.URL, endpoint.SecretCiphertext, endpoint.Status, endpoint.CreatedAt, endpoint.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("webhook endpoint %s does not belong to client %s", endpoint.ID, clientID)
		}
		if _, err := q.Exec(ctx, `DELETE FROM webhook_endpoint_events WHERE endpoint_id=$1`, endpoint.ID); err != nil {
			return err
		}
		for _, eventType := range endpoint.EventTypes {
			if _, err := q.Exec(ctx, `
				INSERT INTO webhook_endpoint_events (endpoint_id, event_type, created_at)
				VALUES ($1,$2,$3)
			`, endpoint.ID, eventType, endpoint.UpdatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s Store) ListEndpointsByClient(ctx context.Context, clientID string) ([]model.WebhookEndpoint, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, client_id, url, secret_ciphertext, status, created_at, updated_at
		FROM webhook_endpoints
		WHERE client_id=$1
		ORDER BY created_at, id
	`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var endpoints []model.WebhookEndpoint
	for rows.Next() {
		var endpoint model.WebhookEndpoint
		if err := rows.Scan(&endpoint.ID, &endpoint.ClientID, &endpoint.URL, &endpoint.SecretCiphertext, &endpoint.Status, &endpoint.CreatedAt, &endpoint.UpdatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range endpoints {
		eventTypes, err := s.EndpointEventTypes(ctx, endpoints[i].ID)
		if err != nil {
			return nil, err
		}
		endpoints[i].EventTypes = eventTypes
	}
	return endpoints, nil
}

func (s Store) EndpointEventTypes(ctx context.Context, endpointID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT event_type
		FROM webhook_endpoint_events
		WHERE endpoint_id=$1
		ORDER BY event_type
	`, endpointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var eventTypes []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			return nil, err
		}
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, rows.Err()
}

func (s Store) ListSubscribedEndpoints(ctx context.Context, eventType, targetClientID string) ([]model.WebhookEndpoint, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT endpoint.id, endpoint.client_id, endpoint.url, endpoint.secret_ciphertext,
		       endpoint.status, endpoint.created_at, endpoint.updated_at
		FROM webhook_endpoint_events AS subscription
		JOIN webhook_endpoints AS endpoint ON endpoint.id=subscription.endpoint_id
		JOIN delegated_clients AS client ON client.id=endpoint.client_id
		WHERE subscription.event_type=$1
		  AND endpoint.status='active'
		  AND client.status='active'
		  AND ($2='' OR endpoint.client_id=$2)
		ORDER BY endpoint.id
	`, eventType, targetClientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var endpoints []model.WebhookEndpoint
	for rows.Next() {
		var endpoint model.WebhookEndpoint
		if err := rows.Scan(&endpoint.ID, &endpoint.ClientID, &endpoint.URL, &endpoint.SecretCiphertext, &endpoint.Status, &endpoint.CreatedAt, &endpoint.UpdatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}

func (s Store) ListPendingEvents(ctx context.Context, limit int) ([]model.WebhookEvent, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, event_type, COALESCE(target_client_id,''), COALESCE(subject_user_id,''), data, created_at, expanded_at
		FROM webhook_events
		WHERE expanded_at IS NULL
		ORDER BY created_at, id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []model.WebhookEvent
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s Store) CompleteExpansion(ctx context.Context, eventID string, endpointIDs []string, expandedAt int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	sort.Strings(endpointIDs)
	for _, endpointID := range endpointIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_deliveries
				(id,event_id,endpoint_id,status,attempt_count,next_attempt_at,created_at,updated_at)
			VALUES ('whd_' || replace(gen_random_uuid()::TEXT, '-', ''),$1,$2,'pending',0,$3,$3,$3)
			ON CONFLICT (event_id,endpoint_id) DO NOTHING
		`, eventID, endpointID, expandedAt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE webhook_events
		SET expanded_at=COALESCE(expanded_at,$2)
		WHERE id=$1
	`, eventID, expandedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) ClaimDueDeliveries(ctx context.Context, now, leaseUntil int64, limit int) ([]model.WebhookDelivery, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH picked AS (
			SELECT id
			FROM webhook_deliveries
			WHERE (status='pending' AND next_attempt_at<=$1)
			   OR (status='processing' AND lease_until<=$1)
			ORDER BY next_attempt_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		), claimed AS (
			UPDATE webhook_deliveries AS delivery
			SET status='processing', attempt_count=attempt_count+1, lease_until=$2, updated_at=$1
			FROM picked
			WHERE delivery.id=picked.id
			RETURNING delivery.id, delivery.event_id, delivery.endpoint_id,
			          delivery.attempt_count, delivery.created_at
		)
		SELECT claimed.id, claimed.attempt_count, claimed.created_at,
		       event.id, event.event_type, COALESCE(event.target_client_id,''),
		       COALESCE(event.subject_user_id,''), event.data, event.created_at, event.expanded_at,
		       endpoint.id, endpoint.client_id, endpoint.url, endpoint.secret_ciphertext,
		       endpoint.status, endpoint.created_at, endpoint.updated_at
		FROM claimed
		JOIN webhook_events AS event ON event.id=claimed.event_id
		JOIN webhook_endpoints AS endpoint ON endpoint.id=claimed.endpoint_id
		ORDER BY claimed.id
	`, now, leaseUntil, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deliveries []model.WebhookDelivery
	for rows.Next() {
		var delivery model.WebhookDelivery
		var rawData []byte
		if err := rows.Scan(
			&delivery.ID, &delivery.AttemptCount, &delivery.CreatedAt,
			&delivery.Event.ID, &delivery.Event.Type, &delivery.Event.TargetClientID,
			&delivery.Event.SubjectUserID, &rawData, &delivery.Event.CreatedAt, &delivery.Event.ExpandedAt,
			&delivery.Endpoint.ID, &delivery.Endpoint.ClientID, &delivery.Endpoint.URL,
			&delivery.Endpoint.SecretCiphertext, &delivery.Endpoint.Status,
			&delivery.Endpoint.CreatedAt, &delivery.Endpoint.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawData, &delivery.Event.Data); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (s Store) CompleteDelivery(ctx context.Context, deliveryID string, deliveredAt int64, httpStatus int) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status='succeeded', delivered_at=$2, last_http_status=$3, last_error='',
		    lease_until=NULL, updated_at=$2
		WHERE id=$1 AND status='processing'
	`, deliveryID, deliveredAt, httpStatus)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) FailDelivery(ctx context.Context, deliveryID, status string, nextAttemptAt, updatedAt int64, httpStatus *int, detail string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status=$2, next_attempt_at=$3, last_http_status=$4, last_error=$5,
		    lease_until=NULL, updated_at=$6
		WHERE id=$1 AND status='processing'
	`, deliveryID, status, nextAttemptAt, httpStatus, detail, updatedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) DeleteTerminalEventsBefore(ctx context.Context, cutoff int64, limit int) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		WITH candidates AS (
			SELECT event.id
			FROM webhook_events AS event
			WHERE event.expanded_at IS NOT NULL
			  AND NOT EXISTS (
				SELECT 1 FROM webhook_deliveries AS active
				WHERE active.event_id=event.id AND active.status IN ('pending','processing')
			  )
			  AND COALESCE((
				SELECT MAX(terminal.updated_at)
				FROM webhook_deliveries AS terminal
				WHERE terminal.event_id=event.id
			  ), event.expanded_at) < $1
			ORDER BY event.created_at, event.id
			LIMIT $2
		)
		DELETE FROM webhook_events AS event
		USING candidates
		WHERE event.id=candidates.id
	`, cutoff, limit)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) GetEvent(ctx context.Context, eventID string) (*model.WebhookEvent, error) {
	event, err := scanEvent(s.Pool.QueryRow(ctx, `
		SELECT id, event_type, COALESCE(target_client_id,''), COALESCE(subject_user_id,''), data, created_at, expanded_at
		FROM webhook_events
		WHERE id=$1
	`, eventID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &event, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (model.WebhookEvent, error) {
	var event model.WebhookEvent
	var rawData []byte
	if err := row.Scan(&event.ID, &event.Type, &event.TargetClientID, &event.SubjectUserID, &rawData, &event.CreatedAt, &event.ExpandedAt); err != nil {
		return model.WebhookEvent{}, err
	}
	if err := json.Unmarshal(rawData, &event.Data); err != nil {
		return model.WebhookEvent{}, err
	}
	return event, nil
}
