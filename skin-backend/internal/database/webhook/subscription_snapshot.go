package webhook

import (
	"context"

	"github.com/jackc/pgx/v5"
)

const subscriptionSnapshotLockID int64 = 0x4553574542484f4f

// LockSubscriptionSnapshot serializes configuration mutations that can change
// the effective set of active Webhook subscriptions. Call it before mutation.
func LockSubscriptionSnapshot(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, subscriptionSnapshotLockID)
	return err
}

// RefreshSubscriptionSnapshot rebuilds the small derived lookup table inside
// the same transaction as the already locked configuration mutation.
func RefreshSubscriptionSnapshot(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM webhook_active_event_types`); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO webhook_active_event_types (event_type, subscriber_count)
		SELECT subscription.event_type, COUNT(*)
		FROM webhook_endpoint_events AS subscription
		JOIN webhook_endpoints AS endpoint ON endpoint.id=subscription.endpoint_id
		JOIN delegated_clients AS client ON client.id=endpoint.client_id
		WHERE endpoint.status='active' AND client.status='active'
		GROUP BY subscription.event_type
	`)
	return err
}
