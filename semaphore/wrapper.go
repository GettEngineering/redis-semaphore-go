package semaphore

import (
	"github.com/pkg/errors"
)

func WithMutex(lockByKey string, redisClient Redis, safeCode func(), options ...Options) error {
	s, err := create(lockByKey, redisClient, options...)
	if err != nil {
		return errors.Wrapf(err, "failed to create semaphore %v", s.lockByKey)
	}

	resource, err := s.Lock()
	if err != nil {
		return errors.Wrapf(err, "failed to lock semaphore %v", s.lockByKey)
	}

	defer func() {
		err := s.Unlock(resource)
		if err != nil {
			s.options.Logger.WithFields(map[string]interface{}{
				"resource": resource,
				"error":    err,
			}).Log(errLvl, "failed to unlock resource after critical section execution")
		}
	}()

	safeCode() //execute locked function

	return nil
}
