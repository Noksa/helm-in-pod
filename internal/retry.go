package internal

import (
	"time"

	"go.uber.org/multierr"
)

// RetryFunc is a function that can be retried
type RetryFunc func() error

// Retry executes fn up to maxAttempts times, sleeping between attempts
func Retry(maxAttempts int, fn RetryFunc) error {
	var mErr error
	for i := 1; i <= maxAttempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		mErr = multierr.Append(mErr, err)
		if i < maxAttempts {
			time.Sleep(time.Second)
		}
	}
	return mErr
}
