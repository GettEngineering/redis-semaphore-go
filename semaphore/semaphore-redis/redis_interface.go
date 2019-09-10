package semaphoreredis

import (
	"time"
)

//go:generate mockgen -source=./semaphore/semaphore-redis/redis_interface.go -destination=./semaphore/semaphore-redis/mock/redis_interface_mock.go Redis
type Redis interface {
	Set(key string, value interface{}, expiration time.Duration) error
	SetNX(key string, value interface{}, expiration time.Duration) (isSet bool, err error)
	Exists(key string) (bool, error)
	TxPipelined(f func(pipe Pipeline) error) error
	BLPop(timeout time.Duration, keys ...string) (val string, isTimedOut bool, err error)
	LLen(key string) (len int64, err error)
	HSet(key, field, value string) error
	HGetAll(key string) (keyVal map[string]string, err error)
	HExists(key, field string) (isExists bool, err error)
	Del(keys ...string) error
}

type Pipeline interface {
	Del(keys ...string) error
	HDel(key string, fields ...string) error
	RPush(key string, values ...interface{}) error
	PExpire(key string, expiration time.Duration) error
}
