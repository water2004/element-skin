package redisstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *RedisStore) stateKey(token string) string {
	return s.key("state", token)
}

func (s *RedisStore) SetState(ctx context.Context, token string, value map[string]any, ttl time.Duration) error {
	return s.setJSON(ctx, s.stateKey(token), value, ttl)
}

func (s *RedisStore) GetState(ctx context.Context, token string) (map[string]any, error) {
	var out map[string]any
	if err := s.getJSON(ctx, s.stateKey(token), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

var popStateScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value then
	return nil
end
redis.call("DEL", KEYS[1])
return value
`)

func (s *RedisStore) PopState(ctx context.Context, token string) (map[string]any, error) {
	result, err := popStateScript.Run(ctx, s.client, []string{s.stateKey(token)}).Result()
	if err == redis.Nil {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	var raw []byte
	switch value := result.(type) {
	case string:
		raw = []byte(value)
	case []byte:
		raw = value
	default:
		return nil, fmt.Errorf("unexpected state payload type %T", result)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func (s *RedisStore) DeleteState(ctx context.Context, token string) error {
	return s.client.Del(ctx, s.stateKey(token)).Err()
}
