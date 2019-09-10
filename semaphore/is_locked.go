package semaphore

import (
	"github.com/pkg/errors"
)

func (s *semaphore) IsResourceLocked(resource string) (bool, error) {
	s.options.Logger.WithFields(map[string]interface{}{
		"resource": resource,
	}).Log(debugLvl, "received is resource locked request")

	return s.isResourceLocked(resource)
}

func (s *semaphore) isResourceLocked(resource string) (bool, error) {
	isResourceLocked, err := s.redis.client.HExists(s.lockedResourcesName(), resource)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if resource %v exists in locked resources queue", resource)
	}

	s.options.Logger.WithFields(map[string]interface{}{
		"resource":  resource,
		"is_locked": isResourceLocked,
	}).Log(debugLvl, "retrieved from redis resource lock status")

	return isResourceLocked, nil
}
