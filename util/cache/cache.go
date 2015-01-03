package cache

import "time"

type MemoizedFn func(args ...interface{}) interface{}

// Cache result of fn, optionally expiring result.
func Memoize(fn MemoizedFn, args ...interface{}) MemoizedFn {
	seconds := int64(0)

	// Takes one extra optional argument, a timeout in seconds
	if len(args) > 0 {
		seconds = int64(args[0].(int))
	}

	// Get next expiration time
	duration := time.Duration(seconds * 1000 * 1000 * 1000) // time.Duration expects nanoseconds
	expires := time.Now()
	expires.Add(duration)

	var cachedResult interface{}

	return func(args ...interface{}) interface{} {
		now := time.Now()

		// If this is the first time being called or expiration hit, cache result
		if cachedResult == nil || (seconds > 0 && now.After(expires)) {
			cachedResult = fn(args...)

			// Reset expiration timeout
			expires = now.Add(duration)
		}

		return cachedResult
	}
}
