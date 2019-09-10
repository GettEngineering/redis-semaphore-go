package semaphoreredis

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type GoRedisImpl struct {
	Client *redis.Client
}

func NewGoRedisImpl(client *redis.Client) Redis {
	return &GoRedisImpl{Client: client}
}

func (c *GoRedisImpl) Set(key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(key, value, expiration).Err()
}

func (c *GoRedisImpl) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.Client.SetNX(key, value, expiration).Result()
}

func (c *GoRedisImpl) Exists(key string) (bool, error) {
	numExistingKeys, err := c.Client.Exists(key).Result()
	if err != nil {
		return false, err
	}

	return numExistingKeys == 1, nil
}

func (c *GoRedisImpl) TxPipelined(f func(pipe Pipeline) error) error {
	_, err := c.Client.TxPipelined(func(pipeline redis.Pipeliner) error {
		pipe := &goRedisPipelineImpl{
			pipeline: pipeline,
		}

		return f(pipe)
	})

	return err
}

func (c *GoRedisImpl) BLPop(timeout time.Duration, keys ...string) (string, bool, error) {
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

func (c *GoRedisImpl) LLen(key string) (int64, error) {
	return c.Client.LLen(key).Result()
}

func (c *GoRedisImpl) HSet(key, field, value string) error {
	return c.Client.HSet(key, field, value).Err()
}

func (c *GoRedisImpl) HGetAll(key string) (map[string]string, error) {
	return c.Client.HGetAll(key).Result()
}

func (c *GoRedisImpl) HExists(key, field string) (bool, error) {
	return c.Client.HExists(key, field).Result()
}

func (c *GoRedisImpl) Del(keys ...string) error {
	return c.Client.Del(keys...).Err()
}

type goRedisPipelineImpl struct {
	pipeline redis.Pipeliner
}

func (c *goRedisPipelineImpl) Del(keys ...string) error {
	return c.pipeline.Del(keys...).Err()
}

func (c *goRedisPipelineImpl) RPush(key string, values ...interface{}) error {
	return c.pipeline.RPush(key, values...).Err()
}

func (c *goRedisPipelineImpl) HDel(key string, fields ...string) error {
	return c.pipeline.HDel(key, fields...).Err()
}

func (c *goRedisPipelineImpl) PExpire(key string, expiration time.Duration) error {
	return c.pipeline.PExpire(key, expiration).Err()
}
