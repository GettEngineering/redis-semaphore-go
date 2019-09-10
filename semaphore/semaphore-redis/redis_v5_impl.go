package semaphoreredis

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/redis.v5"
)

type RedisV5Impl struct {
	Client *redis.Client
}

func NewRedisV5Client(client *redis.Client) Redis {
	return &RedisV5Impl{Client: client}
}

func (c *RedisV5Impl) Set(key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(key, value, expiration).Err()
}

func (c *RedisV5Impl) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.Client.SetNX(key, value, expiration).Result()
}

func (c *RedisV5Impl) Exists(key string) (bool, error) {
	return c.Client.Exists(key).Result()
}

func (c *RedisV5Impl) TxPipelined(f func(pipe Pipeline) error) error {
	_, err := c.Client.TxPipelined(func(pipeline *redis.Pipeline) error {
		pipe := &redisV5PipelineImpl{
			pipeline: pipeline,
		}

		return f(pipe)
	})

	return err
}

func (c *RedisV5Impl) BLPop(timeout time.Duration, keys ...string) (string, bool, error) {
	keyVal, err := c.Client.BLPop(timeout, keys...).Result()
	if err != nil {
		if err == redis.Nil {
			return "", true, nil
		}
		return "", false, errors.Wrapf(err, "failed to pop available resource from queue")
	}

	if len(keyVal) != 2 {
		return "", false, fmt.Errorf("received unexpected value from redis in response to redis blpop command: %v", keyVal)
	}

	return keyVal[1], false, nil
}

func (c *RedisV5Impl) LLen(key string) (int64, error) {
	return c.Client.LLen(key).Result()
}

func (c *RedisV5Impl) HSet(key, field, value string) error {
	return c.Client.HSet(key, field, value).Err()
}

func (c *RedisV5Impl) HGetAll(key string) (map[string]string, error) {
	return c.Client.HGetAll(key).Result()
}

func (c *RedisV5Impl) HExists(key, field string) (bool, error) {
	return c.Client.HExists(key, field).Result()
}

func (c *RedisV5Impl) Del(keys ...string) error {
	return c.Client.Del(keys...).Err()
}

type redisV5PipelineImpl struct {
	pipeline *redis.Pipeline
}

func (c *redisV5PipelineImpl) Del(keys ...string) error {
	return c.pipeline.Del(keys...).Err()
}

func (c *redisV5PipelineImpl) RPush(key string, values ...interface{}) error {
	return c.pipeline.RPush(key, values...).Err()
}

func (c *redisV5PipelineImpl) HDel(key string, fields ...string) error {
	return c.pipeline.HDel(key, fields...).Err()
}

func (c *redisV5PipelineImpl) PExpire(key string, expiration time.Duration) error {
	return c.pipeline.PExpire(key, expiration).Err()
}
