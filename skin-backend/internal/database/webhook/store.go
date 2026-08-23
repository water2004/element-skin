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

func (s Store) ClaimPendingEvents(ctx context.Context, now, leaseUntil int64, limit int) ([]model.WebhookEvent, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH picked AS (
			SELECT id
			FROM webhook_events
			WHERE expanded_at IS NULL
			  AND (expansion_lease_until IS NULL OR expansion_lease_until<=$1)
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		), claimed AS (
			UPDATE webhook_events AS event
			SET expansion_lease_until=$2,
			    expansion_lease_token='whe_' || replace(gen_random_uuid()::TEXT, '-', '')
			FROM picked
			WHERE event.id=picked.id
			RETURNING event.id, event.event_type, COALESCE(event.target_client_id,''),
			          COALESCE(event.subject_user_id,''), event.data, event.created_at,
			          event.expanded_at, event.expansion_lease_until,
			          event.expansion_lease_token
		)
		SELECT * FROM claimed ORDER BY created_at, id
	`, now, leaseUntil, limit)
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

func (s Store) CompleteExpansions(ctx context.Context, expansions []model.WebhookExpansion, expandedAt int64) (int64, int64, error) {
	if len(expansions) == 0 {
		return 0, 0, nil
	}
	expansions = append([]model.WebhookExpansion(nil), expansions...)
	sort.Slice(expansions, func(i, j int) bool {
		return expansions[i].EventID < expansions[j].EventID
	})
	eventIDs := make([]string, 0, len(expansions))
	leaseTokens := make([]string, 0, len(expansions))
	deliveryEventIDs := make([]string, 0)
	endpointIDs := make([]string, 0)
	for _, expansion := range expansions {
		eventIDs = append(eventIDs, expansion.EventID)
		leaseTokens = append(leaseTokens, expansion.LeaseToken)
		ids := append([]string(nil), expansion.EndpointIDs...)
		sort.Strings(ids)
		previous := ""
		for index, endpointID := range ids {
			if index > 0 && endpointID == previous {
				continue
			}
			deliveryEventIDs = append(deliveryEventIDs, expansion.EventID)
			endpointIDs = append(endpointIDs, endpointID)
			previous = endpointID
		}
	}
	var completedEvents, insertedDeliveries int64
	err := s.Pool.QueryRow(ctx, `
		WITH completion_input AS (
			SELECT *
			FROM unnest($1::TEXT[], $2::TEXT[]) AS input(event_id, lease_token)
		), completed AS (
			UPDATE webhook_events AS event
			SET expanded_at=$3, expansion_lease_until=NULL, expansion_lease_token=NULL
			FROM completion_input AS input
			WHERE event.id=input.event_id
			  AND event.expanded_at IS NULL
			  AND event.expansion_lease_token=input.lease_token
			RETURNING event.id
		), delivery_input AS (
			SELECT *
			FROM unnest($4::TEXT[], $5::TEXT[]) AS input(event_id, endpoint_id)
		), inserted AS (
			INSERT INTO webhook_deliveries
				(id,event_id,endpoint_id,status,attempt_count,next_attempt_at,created_at,updated_at)
			SELECT 'whd_' || replace(gen_random_uuid()::TEXT, '-', ''), input.event_id,
			       input.endpoint_id, 'pending', 0, $3, $3, $3
			FROM delivery_input AS input
			JOIN completed ON completed.id=input.event_id
			ON CONFLICT (event_id,endpoint_id) DO NOTHING
			RETURNING id
		)
		SELECT (SELECT COUNT(*) FROM completed), (SELECT COUNT(*) FROM inserted)
	`, eventIDs, leaseTokens, expandedAt, deliveryEventIDs, endpointIDs).Scan(&completedEvents, &insertedDeliveries)
	if err != nil {
		return 0, 0, err
	}
	return completedEvents, insertedDeliveries, nil
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
			SET status='processing', attempt_count=attempt_count+1, lease_until=$2,
			    lease_token='whl_' || replace(gen_random_uuid()::TEXT, '-', ''), updated_at=$1
			FROM picked
			WHERE delivery.id=picked.id
			RETURNING delivery.id, delivery.event_id, delivery.endpoint_id,
			          delivery.attempt_count, delivery.created_at, delivery.lease_token
		)
		SELECT claimed.id, claimed.attempt_count, claimed.created_at, claimed.lease_token,
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
			&delivery.ID, &delivery.AttemptCount, &delivery.CreatedAt, &delivery.LeaseToken,
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

func (s Store) CompleteDeliveryOutcomes(ctx context.Context, outcomes []model.WebhookDeliveryOutcome) (int64, error) {
	if len(outcomes) == 0 {
		return 0, nil
	}
	outcomes = append([]model.WebhookDeliveryOutcome(nil), outcomes...)
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].DeliveryID < outcomes[j].DeliveryID
	})
	deliveryIDs := make([]string, 0, len(outcomes))
	leaseTokens := make([]string, 0, len(outcomes))
	statuses := make([]string, 0, len(outcomes))
	nextAttemptAt := make([]int64, 0, len(outcomes))
	updatedAt := make([]int64, 0, len(outcomes))
	httpStatuses := make([]int, 0, len(outcomes))
	details := make([]string, 0, len(outcomes))
	deliveredAt := make([]int64, 0, len(outcomes))
	for _, outcome := range outcomes {
		httpStatus := 0
		if outcome.HTTPStatus != nil {
			httpStatus = *outcome.HTTPStatus
		}
		delivered := int64(0)
		if outcome.DeliveredAt != nil {
			delivered = *outcome.DeliveredAt
		}
		deliveryIDs = append(deliveryIDs, outcome.DeliveryID)
		leaseTokens = append(leaseTokens, outcome.LeaseToken)
		statuses = append(statuses, outcome.Status)
		nextAttemptAt = append(nextAttemptAt, outcome.NextAttemptAt)
		updatedAt = append(updatedAt, outcome.UpdatedAt)
		httpStatuses = append(httpStatuses, httpStatus)
		details = append(details, outcome.Detail)
		deliveredAt = append(deliveredAt, delivered)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE webhook_deliveries AS delivery
		SET status=input.status,
		    next_attempt_at=CASE WHEN input.status='succeeded' THEN delivery.next_attempt_at ELSE input.next_attempt_at END,
		    delivered_at=NULLIF(input.delivered_at,0),
		    last_http_status=NULLIF(input.http_status,0),
		    last_error=input.detail,
		    lease_until=NULL,
		    lease_token=NULL,
		    updated_at=input.updated_at
		FROM unnest(
			$1::TEXT[], $2::TEXT[], $3::TEXT[], $4::BIGINT[],
			$5::BIGINT[], $6::INTEGER[], $7::TEXT[], $8::BIGINT[]
		) AS input(delivery_id, lease_token, status, next_attempt_at, updated_at, http_status, detail, delivered_at)
		WHERE delivery.id=input.delivery_id
		  AND delivery.status='processing'
		  AND delivery.lease_token=input.lease_token
	`, deliveryIDs, leaseTokens, statuses, nextAttemptAt, updatedAt, httpStatuses, details, deliveredAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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
		SELECT id, event_type, COALESCE(target_client_id,''), COALESCE(subject_user_id,''),
		       data, created_at, expanded_at, expansion_lease_until,
		       COALESCE(expansion_lease_token,'')
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
	if err := row.Scan(
		&event.ID, &event.Type, &event.TargetClientID, &event.SubjectUserID, &rawData,
		&event.CreatedAt, &event.ExpandedAt, &event.ExpansionLeaseUntil,
		&event.ExpansionLeaseToken,
	); err != nil {
		return model.WebhookEvent{}, err
	}
	if err := json.Unmarshal(rawData, &event.Data); err != nil {
		return model.WebhookEvent{}, err
	}
	return event, nil
}
