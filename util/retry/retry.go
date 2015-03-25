package retry

import (
	"github.com/cenkalti/backoff"

	"crowdstart.io/util/log"
)

func Retry(times int, fn func() error) error {
	b := backoff.NewExponentialBackOff()
	ticker := backoff.NewTicker(b)

	var err error

	tries := 0
	for _ = range ticker.C {
		// Give up after so many tries
		if tries > times {
			ticker.Stop()
			break
		}
		tries = tries + 1

		// Try and do something that might fail, retry if it does.
		if err = fn(); err != nil {
			if tries > 1 {
				// Warn about > 1 retries.
				log.Debug("%v (%d tries, will retry...)", err, tries)
			}
			continue
		}

		// Holy shit it worked
		ticker.Stop()
		break
	}

	return err
}
