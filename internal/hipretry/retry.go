package hipretry

import (
	"time"

	"go.uber.org/multierr"
)

// Func is a function that can be retried
type Func func() error

// Retry executes fn up to maxAttempts times, sleeping between attempts
func Retry(maxAttempts int, fn Func) error {
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
