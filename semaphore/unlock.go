package semaphore

import (
	"fmt"

	"github.com/GettEngineering/redis-semaphore-go/semaphore/semaphore-redis"
	"github.com/pkg/errors"
)

func (s *semaphore) Unlock(resource string) error {
	s.options.Logger.WithFields(map[string]interface{}{
		"resource": resource,
	}).Log(debugLvl, "received unlock request")

	isResourceLocked, err := s.isResourceLocked(resource)
	if err != nil {
		return err
	}

	if !isResourceLocked {
		s.options.Logger.WithFields(map[string]interface{}{
			"resource": resource,
		}).Log(infoLvl, "resource was not locked, no need to unlock")
		return nil // index was not locked - no need to do anything
	}

	return s.unlock(resource)
}

func (s *semaphore) unlock(resources ...string) (err error) {
	pipeErr := s.redis.client.TxPipelined(func(pipe semaphoreredis.Pipeline) error { //execute redis transaction

		err = pipe.HDel(s.lockedResourcesName(), resources...)
		if err != nil {
			return errors.Wrapf(err, "failed to remove resources %+v from locked queue", resources)
		}

		var resourcesAsInterface []interface{}

		for _, resource := range resources {
			resourcesAsInterface = append(resourcesAsInterface, resource)
		}

		err = pipe.RPush(s.availableQueueName(), resourcesAsInterface...)
		if err != nil {
			return errors.Wrapf(err, "failed to add resources %+v to available queue", resources)
		}

		return s.updateExpirationTime(pipe)
	})

	if pipeErr != nil {
		return pipeErr
	}

	s.options.Logger.WithFields(map[string]interface{}{
		"resources": fmt.Sprintf("%+v", resources),
	}).Log(infoLvl, "resources unlocked successfully")

	return nil
}
