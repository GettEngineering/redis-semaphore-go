package semaphore

import (
	"github.com/pkg/errors"
)

func (s *semaphore) GetNumAvailableResources() (int64, error) {
	s.options.Logger.Log(debugLvl, "received get num available resources request")

	isSemaphoreExists, err := s.redis.client.Exists(s.name())
	if err != nil {
		return 0, errors.Wrapf(err, "failed to check if semaphore exists while getting num available resources")
	}

	if !isSemaphoreExists { //semaphore does not exists - return initial value
		s.options.Logger.Log(debugLvl, "semaphore does not exists in redis - all resources are free")
		return s.options.MaxParallelResources, nil
	}

	lenQueue, err := s.redis.client.LLen(s.availableQueueName())
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get length of available resources queue")
	}

	s.options.Logger.WithFields(map[string]interface{}{
		"num_available_resources": lenQueue,
	}).Log(debugLvl, "retrieved num available resources for semaphore")

	return lenQueue, nil
}
