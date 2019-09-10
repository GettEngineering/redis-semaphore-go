# **redis-semaphore**

<br> Implements a semaphore using redis commands. The semaphore is blocking, not polling, and has a fair queue serving processes on a first-come, first-serve basis. </br>
<br> Implementation based on Redis BLPOP ability to block execution until queue is not empty or timeout reached. </br>

### **Redis Client**

redis-semaphore requires redis client provided by the user.
<br> It is not dependant on specific redis version, and can accept any implementation that satisfies its Redis interface. </br>
<br> Implementations of `go_redis` && `redis.v5` clients are already given in this repository for your convenience. </br>
<br> Providing nil object will result in validation error. </br>

### **Redis Keys**

redis-semaphore uses 4 keys to maintain semaphore lifecycle:

1. **name** - derived by the binding key given by user. Semaphores are separated in Redis by their names

2. **version** - In case of future possible updates & fixes, version key enables to differentiating between old and updated clients

3. **available resources queue name** - represents the queue name in redis holding list of free locks to use

4. **locked resources set name** - represents key in redis which under it all used locks and their expiration time will be stored

### **Num Connections**

due to the blocking nature of `blpop` command, note that it's very important to set size of redis connections pool that is higher
than number of expected concurrent locks at worst case.
<br> Exhausting all redis connections will result in a deadlock. </br>

### **Options**

#### **Logging**

redis-semaphore provides logging mechanism to enable monitoring in case needed.
<br> It is not dependant on specific log tool, and can accept any implementation that satisfies its Logger interface. </br>
<br> Note that Logger interface should support 3 types of log levels:
    <ul>
    <li> a. `Error` (0) - show only non blocking errors (errors that will not terminate semaphore process) </li>
    <li> b. `Info`  (1) - log only critical information (lock/unlock succeeded/failed, etc) </li>
    <li> c. `Debug` (2) - verbose, include internal steps </li>
    </ul>
<br> Implementation of `logrus` client is already given in this repository for your convenience. </br>
<br> logger is optional. In case user have no need for log, do not pass it in options </br>

#### **Settings**

The semaphore uses 4 settings to determine it's behavior, each of them can be overridden:
       
1. **`Expiration`** - redis-semaphore must have an expiration time to ensure that after a while all evidence of the semaphore will disappear and your redis server will not be cluttered with unused keys.
                      Also, it represents the maximum amount of time mutual exclusion is guaranteed. Value is set to 1 minute by default.

1. **`TryLockTimeout`** - each lock operation must be bounded by max running time and cannot block execution indefinitely. value is set to 30 seconds by default. This setting can be overridden to any duration between 1 second and semaphore expiration time.

2. **`MaxParallelResources`** - redis-semaphore allows to define a set number of processes inside the semaphore-protected block (1 by default). All those processes can run in the critical section simultaneously.

3. **`LockAttempts`** - user can choose to retry acquiring lock if timeout reached. All attempts will have the same timeout. Number of attempts is 1 be default (no retries).

### **Usage**

#### **Creating New Semaphore**

```
bindingKey = "my_lock_key"
redisClient := semaphoreredis.NewRedisV5Client(redis.NewClient(&redis.Options{Addr: "localhost:6379"}))
logger := semaphorelogger.NewLogrusLogger(logrus.New(), semaphorelogger.LogLevelInfo, bindingKey)
overrideSettings := semaphore.Settings{
    TryLockTimeout:       20 * time.Second,
    LockAttempts:         2,
    MaxParallelResources: 1,
}

s, err := semaphore.New(bindingKey, redisClient, logger, overrideSettings)
```

<br> Creates a new Semaphore. Mandatory params are binding key and Redis client. Optional params are logger and overrides to the default settings. Validation error will be returned on invalid params. </br>
<br> After semaphore is created, its settings cannot be modified. If you wish to alter semaphore setting, it would require creating and new object.
     Note that creating multiple semaphores with the same binding key but different `MaxParallelResources` setting will have no effect. The setting of the first semaphore that will acquire lock will be applied until this semaphore will be expired. </br>

#### **Lock & Unlock**

```
token, err := s.Lock()
isLockUsed, err := s.IsResourceLocked(token) //isLockUsed = true
numFreeLocks, err := s.GetNumAvailableResources() //numFreeLocks = MaxParallelResources - 1
err := s.Unlock(token) //don't forget this!
isLockUsed, err := s.IsResourceLocked(token) //isLockUsed = false
numFreeLocks, err := s.GetNumAvailableResources() //numFreeLocks = MaxParallelResources
```

<br> redis-semaphore enables separate lock & unlock operations. </br> 
<br> Performing lock operation on the Semaphore creates all it's keys in redis if used for the first time or expired, and checks for expired locks otherwise (see expired resources section). </br> 
<br> Lock function returns unique uuid representing the acquired lock. This string should be given as parameter to unlock function when we want to release the lock. </br>
<br> Resource will be locked until will be freed by unlocking it, or until semaphore will expire. </br>
<br> Performing lock or unlock oprations resets the semaphore's expiration time. </br>

#### **Execute With Mutex**
```
WithMutex(lockByKey string, redisClient Redis, logger Logger, safeCode func(), settings ...Settings) error
```
Wrapper for encapsulating semaphore internal implementation. Mandatory params are binding key, Redis client and block of code to run. Optional params are logger and settings overrides.
<br> Function will create new semaphore, acquire lock, run function in critical section, and then release lock.
     If error occurred while running code block, unlock procedure will run all the same. </br>

#### **Custom Timeout**
```
token, err := s.LockWithCustomTimeout(5 * time.Second)
```
User can choose to acquire lock using the same Semaphore but with alternating timeout for each lock operation. The custom timeout is subjects to the same limitations as `TryLockTimeout` parameter.
Providing invalid timeout will result in validation error. 

### **Expired Resources**

There are possible cases where non expired Semaphore will contain locks that passed their expiration time. 
The main reason for that is the extension of the Semaphore's expiration upon lock & unlock operations.
Before every lock operation, expired resources (if exists) will be cleaned up and returned to available locks queue.

### **Trying Lock On Expired Semaphore**

Note that as opposed to locking algorithms that uses polling, in case semaphore expires while process awaits in the queue, it will be not possible to acquire lock!
Client will have to wait until timeout will be reached and then he will be able to lock successfully at the next attempt.