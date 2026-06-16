package probe

import (
	"context"
	"errors"
	"strconv"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
)

const (
	defaultIntervalKey = "fallback_probe_interval"
	defaultInterval    = 10 * time.Minute
	minInterval        = time.Minute
)

type IntervalReader interface {
	Get(ctx context.Context, key, fallback string) (string, error)
}

func ReadInterval(ctx context.Context, reader IntervalReader) time.Duration {
	if reader == nil {
		return defaultInterval
	}
	raw, err := reader.Get(ctx, defaultIntervalKey, strconv.Itoa(int(defaultInterval/time.Second)))
	if err != nil {
		return defaultInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultInterval
	}
	d := time.Duration(n) * time.Second
	if d < minInterval {
		return minInterval
	}
	return d
}

func RunLoop(ctx context.Context, db *database.DB, redis redisstore.Store, reader IntervalReader) {
	if db == nil || redis == nil {
		return
	}
	svc := New(db, redis)
	loopOnce := func() {
		if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			// best-effort; failure to probe should not crash the loop
		}
	}
	loopOnce()
	for {
		interval := ReadInterval(ctx, reader)
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			loopOnce()
		}
	}
}
