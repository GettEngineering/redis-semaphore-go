package semaphore

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gtforge/redis-semaphore-go/semaphore/semaphore-logger"

	"github.com/pkg/errors"

	"github.com/tylerb/gls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSemaphore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "semaphore suit")
}

var _ = Describe("semaphore Tests", func() {

	var (
		s         *semaphore
		logger    Logger
		redis     testRedis
		lockByKey string
		err       error
	)

	BeforeEach(func() {
		redis = redisClient
	})

	AfterEach(func() {
		Expect(redis.FlushAll()).To(Succeed())
	})

	var _ = Describe(".New", func() {

		var (
			res       *semaphore
			overrides []Options
		)

		JustBeforeEach(func() {
			res, err = create(lockByKey, redis, overrides...)
		})

		Context("when lock by key is empty", func() {
			BeforeEach(func() {
				lockByKey = ""
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("lock by key field must be non empty"))
				Expect(res).To(BeZero())
			})
		})

		Context("when lock by key is not empty", func() {
			BeforeEach(func() {
				lockByKey = "semaphore_constructor_test"
			})

			Context("when override Options is empty struct", func() {
				BeforeEach(func() {
					overrides = []Options{}
				})

				It("should create semaphore with default Options", func() {
					Expect(err).To(Succeed())
					Expect(res.options).To(Equal(defaultOptions))
				})

				It("should not create any key in redis", func() {
					for _, key := range res.redis.keys {
						exists, err := redis.Exists(key)
						Expect(err).To(Succeed())
						Expect(exists).To(BeFalse())
					}
				})
			})

			Context("when given more than one override Options", func() {
				BeforeEach(func() {
					overrides = []Options{{}, {MaxParallelResources: 3}}
				})

				It("should ignore all overrides except the first", func() {
					Expect(err).To(Succeed())
					Expect(res.options).To(Equal(defaultOptions))
				})
			})

			Context("when given exactly one non empty override Options", func() {

				var overrideSetting = Options{
					Expiration:           1 * time.Minute,
					TryLockTimeout:       30 * time.Second,
					MaxParallelResources: 3,
					LockAttempts:         2,
					Logger:               semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey),
				}

				BeforeEach(func() {
					overrides = []Options{overrideSetting}
				})

				Context("when overriding expiration with invalid value", func() {
					BeforeEach(func() {
						overrides[0].Expiration = time.Millisecond
					})

					It("should return error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("expiration time must be at least 1 second"))
						Expect(res).To(BeZero())
					})
				})

				Context("when overriding expiration with valid value", func() {
					BeforeEach(func() {
						overrides[0].Expiration = overrideSetting.Expiration
					})

					Context("when overriding try lock timeout with invalid value", func() {
						BeforeEach(func() {
							overrides[0].TryLockTimeout = overrides[0].Expiration + 1
						})

						It("should return error", func() {
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("try lock timeout must be at least 1 second and smaller or equal to semaphore Expiration time"))
							Expect(res).To(BeZero())
						})
					})

					Context("when overriding try lock timeout with valid value", func() {

						Context("when overriding max parallel resources with invalid value", func() {
							BeforeEach(func() {
								overrides[0].MaxParallelResources = -2
							})

							It("should return error", func() {
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("max parallel resources setting must be positive number"))
								Expect(res).To(BeZero())
							})
						})

						Context("when overriding max parallel resources with valid value", func() {
							BeforeEach(func() {
								overrides[0].MaxParallelResources = overrideSetting.MaxParallelResources
							})

							Context("when overriding lock attempts with invalid value", func() {
								BeforeEach(func() {
									overrides[0].LockAttempts = -1
								})

								It("should return error", func() {
									Expect(err).To(HaveOccurred())
									Expect(err.Error()).To(ContainSubstring("lock attempts setting must be positive number"))
									Expect(res).To(BeZero())
								})
							})

							Context("when overriding lock attempts with valid value", func() {
								BeforeEach(func() {
									overrides[0].LockAttempts = overrideSetting.LockAttempts
								})

								Context("when not providing logger", func() {
									BeforeEach(func() {
										overrides[0].Logger = nil
									})

									It("should create semaphore with override options and empty logger", func() {
										Expect(err).To(Succeed())
										Expect(res.options.TryLockTimeout).To(Equal(overrideSetting.TryLockTimeout))
										Expect(res.options.LockAttempts).To(Equal(overrideSetting.LockAttempts))
										Expect(res.options.MaxParallelResources).To(Equal(overrideSetting.MaxParallelResources))
										Expect(res.options.Logger).To(Equal(semaphorelogger.NewEmptyLogger()))

									})
								})

								Context("when providing logger", func() {
									BeforeEach(func() {
										overrides[0].Logger = overrideSetting.Logger
									})

									It("should create semaphore with override options & logger", func() {
										Expect(err).To(Succeed())
										Expect(res.options).To(Equal(overrideSetting))
									})
								})
							})
						})
					})
				})
			})
		})
	})

	var _ = Describe(".WithMutex", func() {

		var (
			override              Options
			funcDuration          time.Duration
			functionCalledCounter int64
			numLocksToPerform     int64
			wg                    sync.WaitGroup
			done                  chan bool
			resource              string
		)

		BeforeEach(func() {
			lockByKey = "with_mutex_test_key"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			override = Options{TryLockTimeout: time.Second, Expiration: 3 * time.Second, MaxParallelResources: 5, Logger: logger}
			functionCalledCounter = 0
			done = make(chan bool)
			wg = sync.WaitGroup{}
		})

		BeforeEach(func() {
			s, err = create(lockByKey, redis, override)
			Expect(err).To(Succeed())
		})

		JustBeforeEach(func() {
			gls.Go(func() {
				time.Sleep(50 * time.Millisecond)
				err = WithMutex(lockByKey, redis, func() { atomic.AddInt64(&functionCalledCounter, 1) }, []Options{override}...)
				for i := 0; i < int(atomic.LoadInt64(&numLocksToPerform)); i++ {
					wg.Done()
				}
			})
		})

		AfterEach(func() {
			Expect(redis.FlushAll()).To(Succeed())
		})

		Context("when performing only a single lock", func() {
			BeforeEach(func() {
				addLocks(&wg, &numLocksToPerform, 1)
			})

			It("should lock, run func, unlock and return no error", func() {
				wg.Wait()
				Expect(err).To(Succeed())
				Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform)))
				validateNumFreeLocks(s, s.options.MaxParallelResources)
			})
		})

		Context("when all locks are taken", func() {
			BeforeEach(func() {
				for i := 0; i < int(s.options.MaxParallelResources); i++ {
					resource, err = s.Lock()
					Expect(err).To(Succeed())
					atomic.AddInt64(&functionCalledCounter, 1)
				}
				addLocks(&wg, &numLocksToPerform, s.options.MaxParallelResources)
			})

			Context("when no lock released while waiting to acquire lock", func() {
				BeforeEach(func() {
					// do nothing
				})

				It("should not lock the last function and return timeout error", func() {
					wg.Wait()
					Expect(err).To(HaveOccurred())
					Expect(errors.Cause(err)).To(Equal(TimeoutError))
				})
			})

			Context("when one lock released while waiting to acquire lock", func() {
				JustBeforeEach(func() {
					Expect(s.Unlock(resource)).To(Succeed())
					atomic.AddInt64(&functionCalledCounter, -1)
				})

				It("should lock, run func, unlock and return no error", func() {
					wg.Wait()
					Expect(err).To(Succeed())
					Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform)))
					validateNumFreeLocks(s, 1)
				})
			})
		})

		Context("when function duration < try lock timeout time", func() {
			BeforeEach(func() {
				funcDuration = s.options.TryLockTimeout / 2
			})

			Context("when num simultaneous locks <= max parallel resources", func() {
				BeforeEach(func() {
					runLocksInParallel(s, &wg, funcDuration, &numLocksToPerform, &functionCalledCounter, s.options.MaxParallelResources, done)
				})

				It("should lock, run func, unlock and return no error", func() {
					for i := 0; i < int(atomic.LoadInt64(&numLocksToPerform))-1; i++ {
						<-done
					}
					Expect(err).To(Succeed())
					Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform)))
					validateNumFreeLocks(s, s.options.MaxParallelResources)
				})
			})

			Context("when num simultaneous locks > max parallel resources", func() {
				BeforeEach(func() {
					runLocksInParallel(s, &wg, funcDuration, &numLocksToPerform, &functionCalledCounter, s.options.MaxParallelResources+1, done)
				})

				It("should not lock the last function and return timeout error", func() {
					for i := 0; i < int(atomic.LoadInt64(&numLocksToPerform))-1; i++ {
						<-done
					}
					Expect(err).To(HaveOccurred())
					Expect(errors.Cause(err)).To(Equal(TimeoutError))
					Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform) - 1))
					validateNumFreeLocks(s, s.options.MaxParallelResources)
				})
			})
		})

		Context("when function duration > try lock timeout time", func() {
			BeforeEach(func() {
				funcDuration = s.options.TryLockTimeout * 2
			})

			Context("when num simultaneous locks <= max parallel resources", func() {
				BeforeEach(func() {
					runLocksInParallel(s, &wg, funcDuration, &numLocksToPerform, &functionCalledCounter, s.options.MaxParallelResources-1, done)
				})

				It("should not lock the last function and return timeout error", func() {
					for i := 0; i < int(atomic.LoadInt64(&numLocksToPerform))-1; i++ {
						<-done
					}
					Expect(err).To(Succeed())
					Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform)))
					validateNumFreeLocks(s, s.options.MaxParallelResources)
				})
			})

			Context("when num simultaneous locks > max parallel resources", func() {
				BeforeEach(func() {
					runLocksInParallel(s, &wg, funcDuration, &numLocksToPerform, &functionCalledCounter, s.options.MaxParallelResources+1, done)
				})

				It("should not lock the last function and return timeout error", func() {
					for i := 0; i < int(atomic.LoadInt64(&numLocksToPerform))-1; i++ {
						<-done
					}
					Expect(err).To(HaveOccurred())
					Expect(errors.Cause(err)).To(Equal(TimeoutError))
					Expect(atomic.LoadInt64(&functionCalledCounter)).To(Equal(atomic.LoadInt64(&numLocksToPerform) - 1))
					validateNumFreeLocks(s, s.options.MaxParallelResources)
				})
			})
		})
	})

	Describe(".Lock", func() {

		var (
			numLockedResources int64
			res                string
		)

		BeforeEach(func() {
			lockByKey = "lock_test_key"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			s, err = create(lockByKey, redis, []Options{{TryLockTimeout: time.Second, Expiration: 2 * time.Second, MaxParallelResources: 3, LockAttempts: 2, Logger: logger}}...)
			Expect(err).To(Succeed())
		})

		BeforeEach(func() {
			numLockedResources = 0
		})

		JustBeforeEach(func() {
			res, err = s.Lock()
			numLockedResources++
		})

		Context("when we use this semaphore for the first time", func() {
			BeforeEach(func() {
				numLockedResources = 0
			})

			It("should lock resource and return no error", func() {
				Expect(err).To(Succeed())
				validateSuccessfulLock(s, redis, res, numLockedResources)
			})
		})

		Context("when this semaphore is already in use", func() {

			Context("when there is at least one free resource", func() {
				BeforeEach(func() {
					for i := 0; i < int(s.options.MaxParallelResources)-1; i++ {
						_, err = s.Lock()
						Expect(err).To(Succeed())
						numLockedResources++
					}
				})

				It("should lock resource and return no error", func() {
					Expect(err).To(Succeed())
					validateSuccessfulLock(s, redis, res, numLockedResources)
				})
			})

			Context("when all resources are taken and not expired", func() {
				BeforeEach(func() {
					for i := 0; i < int(s.options.MaxParallelResources); i++ {
						_, err = s.Lock()
						Expect(err).To(Succeed())
						numLockedResources++
					}
				})

				It("should reach timeout and return timeout error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(TimeoutError))
					Expect(res).To(BeZero())
				})
			})

			Context("when reach timeout on first attempt but some resources expired and succeed on second attempt", func() {
				BeforeEach(func() {
					for i := 0; i < int(s.options.MaxParallelResources); i++ {
						_, err = s.Lock()
						Expect(err).To(Succeed())
						numLockedResources++
						time.Sleep(s.options.Expiration / time.Duration(s.options.MaxParallelResources+1))
					}
				})

				It("should release expired resources, lock resource and return no error", func() {
					Expect(err).To(Succeed())
					validateSuccessfulLock(s, redis, res, numLockedResources-2) //two resources will be expired after first attempt
				})
			})
		})
	})

	Describe(".LockWithCustomTimeout", func() {

		var (
			timeout time.Duration
			res     string
		)

		BeforeEach(func() {
			lockByKey = "lock_with_custom_timeout_test_key"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			s, err = create(lockByKey, redis, Options{Logger: logger})
			Expect(err).To(Succeed())
		})

		JustBeforeEach(func() {
			res, err = s.LockWithCustomTimeout(timeout)
		})

		Context("when timeout is smaller than 1 second", func() {
			BeforeEach(func() {
				timeout = 0
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("try lock timeout must be at least 1 second and smaller or equal to semaphore Expiration time"))
			})
		})

		Context("when timeout is greater than semaphore Expiration time", func() {
			BeforeEach(func() {
				timeout = s.options.Expiration + 1
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("try lock timeout must be at least 1 second and smaller or equal to semaphore Expiration time"))
			})
		})

		Context("when timeout is a positive duration smaller than semaphore Expiration time", func() {
			BeforeEach(func() {
				timeout = s.options.Expiration - 1
			})

			It("should perform lock operation", func() {
				Expect(err).To(Succeed())
				Expect(res).To(Not(BeEmpty()))
			})
		})
	})

	Describe(".Unlock", func() {

		var resource string

		BeforeEach(func() {
			lockByKey = "unlock_test_key"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			s, err = create(lockByKey, redis, []Options{{MaxParallelResources: 3, Logger: logger}}...)
			Expect(err).To(Succeed())
		})

		JustBeforeEach(func() {
			err = s.Unlock(resource)
		})

		Context("when resource is not locked", func() {
			BeforeEach(func() {
				// do nothing
			})

			It("should do nothing and return no error", func() {
				Expect(err).To(Succeed())
			})
		})

		Context("when resource is locked", func() {
			BeforeEach(func() {
				resource, err = s.Lock()
				Expect(err).To(Succeed())
			})

			It("should unlock resource and return no error", func() {
				Expect(err).To(Succeed())
				validateSuccessfulUnLock(s, redis, resource, 0)
			})
		})
	})

	Describe(".IsResourceLocked", func() {

		var (
			checkedResource, lockedResource string
			res                             bool
		)

		BeforeEach(func() {
			lockByKey = "is_resource_locked_test"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			s, err = create(lockByKey, redis, []Options{{TryLockTimeout: time.Second, Expiration: time.Second, Logger: logger}}...)
			Expect(err).To(Succeed())
		})

		JustBeforeEach(func() {
			res, err = s.IsResourceLocked(checkedResource)
		})

		Context("when semaphore not being locked yet", func() {
			BeforeEach(func() {
				// do nothing
			})

			It("should return false", func() {
				Expect(err).To(Succeed())
				Expect(res).To(BeFalse())
			})
		})

		Context("when there are locked resources", func() {
			BeforeEach(func() {
				lockedResource, err = s.Lock()
				Expect(err).To(Succeed())
			})

			Context("when locked resource != checked resource", func() {
				BeforeEach(func() {
					checkedResource = lockedResource + "1"
				})

				It("should return false", func() {
					Expect(err).To(Succeed())
					Expect(res).To(BeFalse())
				})
			})

			Context("when locked resource = checked resource", func() {
				BeforeEach(func() {
					checkedResource = lockedResource
				})

				Context("when semaphore name key has expired", func() {

					Context("when semaphore locked queue key has not expired yet", func() {
						BeforeEach(func() {
							time.Sleep(s.options.Expiration)
						})

						It("should return true", func() {
							Expect(err).To(Succeed())
							Expect(res).To(BeTrue())
						})
					})

					Context("when semaphore locked queue key has also expired", func() {
						BeforeEach(func() {
							time.Sleep(s.options.Expiration * 2)
						})

						It("should return false", func() {
							Expect(err).To(Succeed())
							Expect(res).To(BeFalse())
						})
					})
				})

				Context("when semaphore name key has not expired", func() {
					BeforeEach(func() {
						// do nothing
					})

					It("should return true", func() {
						Expect(err).To(Succeed())
						Expect(res).To(BeTrue())
					})
				})
			})
		})
	})

	Describe(".GetNumAvailableResources", func() {

		var (
			semaphoreSize, numLockedResources int64
			res                               int64
		)

		BeforeEach(func() {
			semaphoreSize = 4
			numLockedResources = 2
		})

		BeforeEach(func() {
			lockByKey = "num_available_resources_test_key"
			logger = semaphorelogger.NewLogrusLogger(nil, debugLvl, lockByKey)
			s, err = create(lockByKey, redis, []Options{{TryLockTimeout: time.Second, Expiration: time.Second, MaxParallelResources: semaphoreSize, Logger: logger}}...)
			Expect(err).To(Succeed())
		})

		JustBeforeEach(func() {
			res, err = s.GetNumAvailableResources()
		})

		Context("when semaphore not locked yet", func() {
			BeforeEach(func() {
				// do nothing
			})

			It("should return semaphore size", func() {
				Expect(err).To(Succeed())
				Expect(res).To(Equal(semaphoreSize))
			})
		})

		Context("when semaphore locked before", func() {
			BeforeEach(func() {
				for i := 0; i < int(numLockedResources); i++ {
					_, err = s.Lock()
					Expect(err).To(Succeed())
				}
			})

			Context("when semaphore has not expired", func() {
				BeforeEach(func() {
					// do nothing
				})

				It("should return number of free resources", func() {
					Expect(err).To(Succeed())
					Expect(res).To(Equal(semaphoreSize - numLockedResources))
				})
			})

			Context("when semaphore has expired", func() {
				BeforeEach(func() {
					time.Sleep(s.options.Expiration)
				})

				It("should return semaphore size", func() {
					Expect(err).To(Succeed())
					Expect(res).To(Equal(semaphoreSize))
				})
			})
		})
	})
})

func runLocksInParallel(s *semaphore, wg *sync.WaitGroup, funcDuration time.Duration, numLocksToPerformAddr, functionCalledCounterAddr *int64, numLocksToPerform int64, done chan bool) {
	addLocks(wg, numLocksToPerformAddr, numLocksToPerform)

	for i := 0; i < int(numLocksToPerform)-1; i++ {
		gls.Go(func() {
			defer GinkgoRecover()
			Expect(WithMutex(s.lockByKey, s.redis.client, func() { time.Sleep(funcDuration); atomic.AddInt64(functionCalledCounterAddr, 1); wg.Wait() }, s.options)).To(Succeed())
			done <- true
		})
	}
}

func addLocks(wg *sync.WaitGroup, numLocksToPerformAddr *int64, numLocks int64) {
	atomic.StoreInt64(numLocksToPerformAddr, numLocks)
	wg.Add(int(numLocks))
}

func validateNumFreeLocks(s *semaphore, expectedNumFreeLocks int64) {
	numFreeLocks, err := s.GetNumAvailableResources()
	Expect(err).To(Succeed())
	Expect(numFreeLocks).To(Equal(expectedNumFreeLocks))
}

func validateSuccessfulLock(s *semaphore, redis testRedis, resource string, numLockedResources int64) {
	//check resource added to locked resources queue
	isResourceLocked, err := s.IsResourceLocked(resource)
	Expect(err).To(Succeed())
	Expect(isResourceLocked).To(BeTrue())

	//check resource deleted from available resources queue
	numAvailableResources, err := s.GetNumAvailableResources()
	Expect(err).To(Succeed())
	Expect(numAvailableResources).To(Equal(s.options.MaxParallelResources - numLockedResources))

	//check all redis keys Expiration time updated
	validateExpirationTime(s, redis, numLockedResources)
}

func validateSuccessfulUnLock(s *semaphore, redis testRedis, resource string, numLockedResources int64) {
	//check resource deleted from locked resources queue
	isResourceLocked, err := s.IsResourceLocked(resource)
	Expect(err).To(Succeed())
	Expect(isResourceLocked).To(BeFalse())

	//check resource added to available resources queue
	numAvailableResources, err := s.GetNumAvailableResources()
	Expect(err).To(Succeed())
	Expect(numAvailableResources).To(Equal(s.options.MaxParallelResources - numLockedResources))

	//check all redis keys Expiration time updated
	validateExpirationTime(s, redis, numLockedResources)
}

func validateExpirationTime(s *semaphore, redis testRedis, numLockedResources int64) {
	for _, key := range s.redis.keys {
		ttl, err := redis.TTL(key)
		Expect(err).To(Succeed())

		if (key == s.availableQueueName() && numLockedResources == s.options.MaxParallelResources) || (key == s.lockedResourcesName() && numLockedResources == 0) { //no items in queue so key deleted
			Expect(ttl.Seconds()).To(Equal(float64(-2))) //not exists
		} else if key == s.lockedResourcesName() {
			Expect(ttl.Seconds()).To(Equal(s.options.Expiration.Seconds() * 2))
		} else {
			Expect(ttl.Seconds()).To(Equal(s.options.Expiration.Seconds()))
		}
	}
}
