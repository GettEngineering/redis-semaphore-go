package semaphore

import (
	"github.com/GettEngineering/redis-semaphore-go/semaphore/semaphore-redis"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

const (
	mutexExistsValue = "v"
	semaphoreVersion = "1.0"
)

func (s *semaphore) init() (err error) {
	isNewSemaphore, err := s.redis.client.SetNX(s.name(), mutexExistsValue, s.options.Expiration)
	if err != nil {
		return errors.Wrapf(err, "failed to setNX semaphore name")
	}

	if isNewSemaphore {
		s.options.Logger.Log(debugLvl, "semaphore is used for the first time or expired, creating keys in redis")
		return s.create()
	} else { //semaphore with this key already exists
		s.options.Logger.Log(debugLvl, "semaphore already exists in redis - checking if should release expired resources")
		return s.releaseExpiredResources()
	}
}

func (s *semaphore) create() error {
	pipeErr := s.redis.client.TxPipelined(func(pipe semaphoreredis.Pipeline) error { //execute redis transaction

		err := pipe.Del(s.lockedResourcesName())
		if err != nil {
			return errors.Wrapf(err, "failed to delete queue of locked resources")
		}

		err = pipe.Del(s.availableQueueName()) //in case last client crushed and did not expire key
		if err != nil {
			return errors.Wrapf(err, "failed to delete queue of available resources")
		}

		var resources []interface{}

		for i := 0; i < int(s.options.MaxParallelResources); i++ {
			resources = append(resources, uuid.NewV4().String())
		}

		err = pipe.RPush(s.availableQueueName(), resources...)
		if err != nil {
			return errors.Wrapf(err, "failed to add %v resources to available resources queue", s.options.MaxParallelResources)
		}

		err = s.redis.client.Set(s.version(), semaphoreVersion, s.options.Expiration)
		if err != nil {
			return errors.Wrapf(err, "failed to set semaphore version")
		}

		return nil
	})

	if pipeErr != nil {
		return pipeErr
	}

	s.options.Logger.Log(debugLvl, "semaphore created in redis successfully")

	return nil
}
