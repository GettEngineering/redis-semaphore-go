package semaphore

import (
	"fmt"
	"time"

	"github.com/GettEngineering/redis-semaphore-go/semaphore/semaphore-redis"

	"github.com/pkg/errors"
)

var TimeoutError = errors.New("reached timeout while trying to acquire lock")

func (s *semaphore) Lock() (string, error) {
	return s.LockWithCustomTimeout(s.options.TryLockTimeout)
}

func (s *semaphore) LockWithCustomTimeout(timeout time.Duration) (string, error) {
	if timeout < time.Second || timeout > s.options.Expiration {
		return "", errors.New("try lock timeout must be at least 1 second and smaller or equal to semaphore Expiration time")
	}

	var (
		isTimedOut bool
		resource   string
		attempts   int64
	)

	s.options.Logger.WithFields(map[string]interface{}{
		"attempts": s.options.LockAttempts,
		"timeout":  timeout,
	}).Log(debugLvl, "received lock request")

	for ok := true; ok; ok = isTimedOut {

		attempts++

		if attempts > s.options.LockAttempts {
			s.options.Logger.WithFields(map[string]interface{}{
				"attempts": s.options.LockAttempts,
				"timeout":  timeout,
			}).Log(infoLvl, "all attempts to acquire lock reached timeout")
			return "", TimeoutError //reached timeout when trying to pop from queue
		}

		err := s.init()
		if err != nil {
			return "", err
		}

		s.options.Logger.WithFields(map[string]interface{}{
			"attempt": attempts,
			"timeout": timeout,
		}).Log(debugLvl, "trying to acquire lock")

		resource, isTimedOut, err = s.redis.client.BLPop(timeout, s.availableQueueName())
		if err != nil {
			return "", errors.Wrapf(err, "failed to pop available resource from queue")
		}

		if isTimedOut {
			s.options.Logger.WithFields(map[string]interface{}{
				"attempt":       attempts,
				"timeout":       timeout,
				"attempts_left": s.options.LockAttempts - attempts,
			}).Log(infoLvl, "reached timeout while trying to acquire lock")
		}
	}

	now := time.Now().UnixNano()

	resource = fmt.Sprintf("%v:%v", resource, now)

	pipeErr := s.redis.client.TxPipelined(func(pipe semaphoreredis.Pipeline) error { //execute redis transaction

		err := s.redis.client.HSet(s.lockedResourcesName(), resource, fmt.Sprint(now)) //value = time of insertion so we would know when it is expired
		if err != nil {
			return errors.Wrapf(err, "failed to add resource %v to locked queue", resource)
		}

		return s.updateExpirationTime(pipe)
	})

	if pipeErr != nil {
		return resource, pipeErr
	}

	s.options.Logger.WithFields(map[string]interface{}{
		"resource": resource,
	}).Log(infoLvl, "resource locked successfully")

	return resource, nil
}

func (s *semaphore) updateExpirationTime(pipe semaphoreredis.Pipeline) error {
	s.options.Logger.WithFields(map[string]interface{}{
		"expiration_time": time.Now().Add(s.options.Expiration).Format("15:04:05.000"),
	}).Log(debugLvl, "update semaphore redis keys Expiration time")

	var err error

	for _, k := range s.redis.keys {

		if k == s.lockedResourcesName() {
			err = pipe.PExpire(k, s.options.Expiration*2) //avoid race condition where semaphore will expire between init and lock
		} else {
			err = pipe.PExpire(k, s.options.Expiration)
		}

		if err != nil {
			return errors.Wrapf(err, "failed to update Expiration time of key %v", k)
		}
	}

	return nil
}
