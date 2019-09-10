package semaphore

import (
	"fmt"
	"time"

	"github.com/gtforge/redis-semaphore-go/semaphore/semaphore-logger"

	"github.com/gtforge/redis-semaphore-go/semaphore/semaphore-redis"

	"github.com/pkg/errors"
)

const (
	errLvl   = semaphorelogger.LogLevelError
	infoLvl  = semaphorelogger.LogLevelInfo
	debugLvl = semaphorelogger.LogLevelDebug
)

type Redis = semaphoreredis.Redis

type Logger = semaphorelogger.Logger

type semaphore struct {
	lockByKey string
	options   Options
	redis     redisImpl
}

type redisImpl struct {
	client Redis
	keys   []string
}

type Options struct {
	TryLockTimeout       time.Duration
	LockAttempts         int64
	MaxParallelResources int64
	Logger               Logger
	Expiration           time.Duration
}

var defaultOptions = Options{
	Expiration:           1 * time.Minute,
	TryLockTimeout:       30 * time.Second,
	LockAttempts:         1,
	MaxParallelResources: 1,
	Logger:               semaphorelogger.NewEmptyLogger(),
}

func New(lockByKey string, redisClient Redis, options ...Options) (Semaphore, error) {
	return create(lockByKey, redisClient, options...)
}

func create(lockByKey string, redisClient Redis, options ...Options) (*semaphore, error) {

	s := &semaphore{
		lockByKey: lockByKey,
		options:   setOptions(options...),
		redis:     redisImpl{client: redisClient},
	}

	err := s.validate()
	if err != nil {
		return nil, errors.Wrapf(err, "error in validating new semaphore")
	}

	s.setRedisKeys()

	s.options.Logger.WithFields(map[string]interface{}{
		"options": fmt.Sprintf("%+v", s.options),
	}).Log(debugLvl, "new semaphore object created successfully")

	return s, nil
}

func setOptions(overrides ...Options) Options {
	options := defaultOptions

	if len(overrides) == 0 {
		return options
	}

	override := overrides[0]

	if override.Expiration != 0 {
		options.Expiration = override.Expiration
	}

	if override.TryLockTimeout != 0 {
		options.TryLockTimeout = override.TryLockTimeout
	}

	if override.MaxParallelResources != 0 {
		options.MaxParallelResources = override.MaxParallelResources
	}

	if override.LockAttempts != 0 {
		options.LockAttempts = override.LockAttempts
	}

	if override.Logger != nil {
		options.Logger = override.Logger
	}

	return options
}

func (s *semaphore) validate() error {
	if s.lockByKey == "" {
		return fmt.Errorf("lock by key field must be non empty")
	}

	if s.redis.client == nil {
		return fmt.Errorf("redis client must be non nil")
	}

	if s.options.Expiration < time.Second {
		return fmt.Errorf("expiration time must be at least 1 second, received %v", s.options.TryLockTimeout)
	}

	if s.options.TryLockTimeout < time.Second || s.options.TryLockTimeout > s.options.Expiration {
		return fmt.Errorf("try lock timeout must be at least 1 second and smaller or equal to semaphore Expiration time, received %v", s.options.TryLockTimeout)
	}

	if s.options.MaxParallelResources <= 0 {
		return fmt.Errorf("max parallel resources setting must be positive number, received %v", s.options.MaxParallelResources)
	}

	if s.options.LockAttempts <= 0 {
		return fmt.Errorf("lock attempts setting must be positive number, received %v", s.options.LockAttempts)
	}

	return nil
}

func (s *semaphore) setRedisKeys() {
	s.redis.keys = append(s.redis.keys, s.name(), s.availableQueueName(), s.lockedResourcesName(), s.version())
}

const (
	namePrefix                = "semaphore"
	availableQueueNamePostfix = "available"
	lockedQueueNamePostfix    = "locked"
	versionPostfix            = "version"
	releaseExpiredPostfix     = "release_expired"
)

func (s *semaphore) name() string {
	return fmt.Sprintf("%v:%v", namePrefix, s.lockByKey)
}

func (s *semaphore) availableQueueName() string {
	return fmt.Sprintf("%v:%v", s.name(), availableQueueNamePostfix)
}

func (s *semaphore) lockedResourcesName() string {
	return fmt.Sprintf("%v:%v", s.name(), lockedQueueNamePostfix)
}

func (s *semaphore) version() string {
	return fmt.Sprintf("%v:%v", s.name(), versionPostfix)
}

func (s *semaphore) releaseExpiredLockName() string {
	return fmt.Sprintf("%v:%v", s.name(), releaseExpiredPostfix)
}
