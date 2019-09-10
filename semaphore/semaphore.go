package semaphore

import "time"

//go:generate mockgen -source=./semaphore/semaphore.go -destination=./semaphore/mock/semaphore_mock.go Semaphore
type Semaphore interface {
	Lock() (string, error)
	LockWithCustomTimeout(timeout time.Duration) (string, error)
	Unlock(resource string) error
	IsResourceLocked(resource string) (bool, error)
	GetNumAvailableResources() (int64, error)
}
