package semaphore

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

const (
	secondaryLockValue      = "v"
	secondaryLockExpiration = 5 * time.Second
)

func (s *semaphore) releaseExpiredResources() error {

	isSecondaryLockAcquired, err := s.redis.client.SetNX(s.releaseExpiredLockName(), secondaryLockValue, secondaryLockExpiration) //releasing expired resources should be done under lock
	if err != nil {
		return errors.Wrapf(err, "failed to setNX secondary lock for releasing expired resources")
	}

	if !isSecondaryLockAcquired {
		s.options.Logger.Log(infoLvl, "semaphore failed to acquire secondary lock (already taken by another process) - skip releasing expired resources")
		return nil //other process is already releasing expired resources
	}

	defer func() {
		err := s.redis.client.Del(s.releaseExpiredLockName())
		if err != nil {
			s.options.Logger.WithFields(map[string]interface{}{
				"error": err,
			}).Log(errLvl, "failed to delete secondary lock while releasing expired resources")
		}

	}() //release secondary lock once finished

	lockedResourcesMap, err := s.redis.client.HGetAll(s.lockedResourcesName())
	if err != nil {
		return errors.Wrapf(err, "failed to get all locked resources")
	}

	if len(lockedResourcesMap) == 0 {
		s.options.Logger.Log(debugLvl, "all semaphore locks are free")
	}

	var expiredResources []string

	for resource, lockedAtStr := range lockedResourcesMap {

		lockedAt, err := strconv.ParseInt(lockedAtStr, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to parse locked resource Expiration time %v as integer", lockedAtStr)
		}

		expireAt := lockedAt + s.options.Expiration.Nanoseconds()

		if expireAt <= (time.Now().UnixNano()) { //resource is using lock more than Expiration time - release index

			expiredResources = append(expiredResources, resource)

			s.options.Logger.WithFields(map[string]interface{}{
				"resource":   resource,
				"locked_at":  time.Unix(0, lockedAt).Format("15:04:05.000"),
				"expired_at": time.Unix(0, expireAt).Format("15:04:05.000"),
			}).Log(infoLvl, "found resource with expired lock - performing unlock")
		} else {
			s.options.Logger.WithFields(map[string]interface{}{
				"resource":       resource,
				"locked_at":      time.Unix(0, lockedAt).Format("15:04:05.000"),
				"will_expire_at": time.Unix(0, expireAt).Format("15:04:05.000"),
			}).Log(infoLvl, "resource is locked and not expired yet")
		}
	}

	if len(expiredResources) > 0 {
		return s.unlock(expiredResources...)
	}

	return nil
}
