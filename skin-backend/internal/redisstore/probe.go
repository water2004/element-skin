package redisstore

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type ProbeSample struct {
	EndpointID int    `json:"endpoint_id"`
	Note       string `json:"note"`
	SessionURL string `json:"session_url"`
	AccountURL string `json:"account_url"`
	ServicesURL string `json:"services_url"`
	CheckedAt  int64  `json:"checked_at"`
	Session    string `json:"session"`
	Account    string `json:"account"`
	Services   string `json:"services"`
}

const ProbeHistoryRetention = 24 * time.Hour

func (s *RedisStore) probeHistoryKey() string {
	return s.key("probe", "history")
}

func (s *RedisStore) AppendProbeSamples(ctx context.Context, samples []ProbeSample, retention time.Duration) error {
	if len(samples) == 0 {
		return nil
	}
	key := s.probeHistoryKey()
	pipe := s.client.Pipeline()
	for _, sample := range samples {
		b, err := json.Marshal(sample)
		if err != nil {
			return err
		}
		pipe.ZAdd(ctx, key, redis.Z{Score: float64(sample.CheckedAt), Member: string(b)})
	}
	cutoff := time.Now().Add(-retention).UnixMilli()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(cutoff, 10))
	if retention > 0 {
		pipe.PExpire(ctx, key, retention+time.Hour)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetProbeHistory(ctx context.Context, since time.Time) ([]ProbeSample, error) {
	min := strconv.FormatInt(since.UnixMilli(), 10)
	values, err := s.client.ZRangeByScore(ctx, s.probeHistoryKey(), &redis.ZRangeBy{Min: min, Max: "+inf"}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]ProbeSample, 0, len(values))
	for _, raw := range values {
		var sample ProbeSample
		if err := json.Unmarshal([]byte(raw), &sample); err != nil {
			return nil, err
		}
		out = append(out, sample)
	}
	return out, nil
}

func (s *RedisStore) InvalidateProbeHistory(ctx context.Context) error {
	return s.client.Del(ctx, s.probeHistoryKey()).Err()
}
