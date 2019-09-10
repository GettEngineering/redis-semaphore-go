package semaphore

import (
	"time"

	"gopkg.in/redis.v5"

	"github.com/gtforge/redis-semaphore-go/semaphore/semaphore-redis"
)

type testRedis interface {
	semaphoreredis.Redis
	TTL(key string) (time.Duration, error)
	FlushAll() error
}

type testRedisWrapper struct {
	semaphoreredis.RedisV5Impl
}

var redisClient testRedis = &testRedisWrapper{RedisV5Impl: semaphoreredis.RedisV5Impl{Client: redis.NewClient(&redis.Options{Addr: "localhost:6379"})}}

func (w *testRedisWrapper) TTL(key string) (time.Duration, error) {
	return w.Client.TTL(key).Result()
}

func (w *testRedisWrapper) FlushAll() error {
	return w.Client.FlushAll().Err()
}
