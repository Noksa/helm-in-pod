package hipretry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net/url"
	"time"

	"go.uber.org/multierr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/noksa/helm-in-pod/internal/logz"
)

// Func is a function that can be retried.
type Func func() error

// BackoffConfig controls the exponential backoff behavior.
type BackoffConfig struct {
	// Base is the initial wait before the second attempt.
	Base time.Duration
	// Cap is the maximum wait per attempt.
	Cap time.Duration
	// Multiplier is the exponential growth factor (e.g. 2.0 doubles each time).
	Multiplier float64
	// MaxAttempts is the total number of tries. A value of zero means unlimited:
	// the loop only stops when fn returns nil, when fn returns a permanent error,
	// or when ctx is canceled. Use unlimited mode only with a context that has a
	// deadline or that you can cancel; otherwise the loop will run forever.
	MaxAttempts int
	// JitterFull enables full-jitter sleeps: sleep = rand[0, cappedExp).
	// Recommended for any workload where multiple processes may retry against
	// a shared backend simultaneously, such as the Kubernetes API server.
	JitterFull bool
}

// DefaultBackoff returns the BackoffConfig used by Retry and RetryWithContext.
// Sleep schedule (no jitter): 1s, 2s, 4s, 8s, 16s, capped at 60s.
func DefaultBackoff() BackoffConfig {
	return BackoffConfig{
		Base:       1 * time.Second,
		Cap:        60 * time.Second,
		Multiplier: 2.0,
		JitterFull: false,
	}
}

// K8sAPIBackoff returns a BackoffConfig tuned for direct Kubernetes API
// calls (Get, Update, Patch, Delete) that may return 429 TooManyRequests,
// 503 ServiceUnavailable, or transient 500s with etcd hints.
//
// Full jitter is enabled to spread retries across concurrent callers and
// avoid amplifying API-server load.
//
// Sleep schedule (with full jitter): rand[0,500ms), rand[0,1s), rand[0,2s),
// rand[0,4s) ... capped at 60s.
func K8sAPIBackoff(maxAttempts int) BackoffConfig {
	return BackoffConfig{
		Base:        500 * time.Millisecond,
		Cap:         60 * time.Second,
		Multiplier:  2.0,
		JitterFull:  true,
		MaxAttempts: maxAttempts,
	}
}

// Retry calls fn up to maxAttempts times, sleeping between attempts using
// the schedule defined by DefaultBackoff. It stops early when fn returns nil
// or when fn returns a permanent error (see isPermanent).
//
// Retry uses context.Background() internally, so the inter-attempt sleep is
// not interruptible by signals or deadlines. Callers that hold a context
// should prefer RetryWithContext.
//
// maxAttempts=0 returns nil without calling fn.
func Retry(maxAttempts int, fn Func) error {
	if maxAttempts == 0 {
		return nil
	}
	cfg := DefaultBackoff()
	cfg.MaxAttempts = maxAttempts
	return RetryWithBackoff(context.Background(), cfg, fn)
}

// RetryWithContext calls fn up to maxAttempts times using DefaultBackoff,
// like Retry, but propagates ctx cancellation into the inter-attempt sleep.
// A canceled or expired context aborts the sleep immediately and returns
// the accumulated error joined with ctx.Err().
//
// maxAttempts=0 returns nil without calling fn.
func RetryWithContext(ctx context.Context, maxAttempts int, fn Func) error {
	if maxAttempts == 0 {
		return nil
	}
	cfg := DefaultBackoff()
	cfg.MaxAttempts = maxAttempts
	return RetryWithBackoff(ctx, cfg, fn)
}

// RetryWithBackoff calls fn with a fully customizable backoff strategy. It:
//   - honors ctx cancellation in the inter-attempt sleep,
//   - stops on permanent errors (see isPermanent),
//   - respects the Retry-After hint from Kubernetes 429/503 responses when
//     it exceeds the calculated backoff.
//
// All errors returned by fn are accumulated and returned together (via
// go.uber.org/multierr) when the loop terminates without success.
//
// Example:
//
//	hipretry.RetryWithBackoff(ctx, hipretry.K8sAPIBackoff(5), fn)
func RetryWithBackoff(ctx context.Context, cfg BackoffConfig, fn Func) error {
	var mErr error

	for attempt := 0; cfg.MaxAttempts == 0 || attempt < cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return multierr.Append(mErr, ctx.Err())
		}

		err := fn()
		if err == nil {
			return nil
		}

		mErr = multierr.Append(mErr, err)

		isLast := cfg.MaxAttempts > 0 && attempt+1 >= cfg.MaxAttempts
		if isLast {
			break
		}

		if isPermanent(err) {
			logz.Host().Debug().
				Err(err).
				Int("attempt", attempt+1).
				Msg("permanent error - skipping remaining retries")
			break
		}

		sleep := nextSleep(cfg, attempt)

		// Respect Retry-After from K8s 429/503 when it exceeds our calculated backoff.
		if serverDelay := retryAfterSeconds(err); serverDelay > sleep {
			sleep = serverDelay
		}

		logz.Host().Warn().
			Err(err).
			Int("attempt", attempt+1).
			Int("max_attempts", cfg.MaxAttempts).
			Dur("retry_in", sleep).
			Msg("transient error - retrying with backoff")

		t := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			t.Stop()
			return multierr.Append(mErr, ctx.Err())
		case <-t.C:
		}
	}

	return mErr
}

// ─── error classification ─────────────────────────────────────────────────────

// isPermanent returns true for Kubernetes API errors that will never recover
// on their own: not-found, forbidden, unauthorized, invalid spec, etc.
// Everything else is treated as potentially transient and will be retried.
//
// context.Canceled is intentionally NOT classified as permanent here. If fn
// returns context.Canceled, it may be from an inner per-call context (e.g. a
// per-request timeout), not from the outer retry context. The outer loop
// handles ctx cancellation via ctx.Err() at the top of each iteration.
//
// Note: HTTP 409 Conflict (optimistic-locking mismatch) is intentionally NOT
// permanent — it resolves once the caller re-reads the resource.
func isPermanent(err error) bool {
	if err == nil {
		return false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isPermanent(urlErr.Err)
	}
	return k8serrors.IsNotFound(err) ||
		k8serrors.IsForbidden(err) ||
		k8serrors.IsUnauthorized(err) ||
		k8serrors.IsInvalid(err) ||
		k8serrors.IsMethodNotSupported(err) ||
		k8serrors.IsRequestEntityTooLargeError(err)
}

// retryAfterSeconds extracts the Retry-After hint from a K8s API status error
// (commonly returned with HTTP 429 or 503). Returns 0 if not present.
func retryAfterSeconds(err error) time.Duration {
	var statusErr *k8serrors.StatusError
	if !errors.As(err, &statusErr) {
		return 0
	}
	if statusErr.Status().Details == nil {
		return 0
	}
	secs := statusErr.Status().Details.RetryAfterSeconds
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// ─── backoff math ─────────────────────────────────────────────────────────────

// nextSleep returns the sleep duration for the given attempt (0-indexed).
//
//	no-jitter:   min(Cap, Base * Multiplier^attempt)
//	full-jitter: rand[0, min(Cap, Base * Multiplier^attempt))
//
// Full jitter is the recommended strategy for high-concurrency workloads
// (AWS "Exponential Backoff and Jitter" approach).
func nextSleep(cfg BackoffConfig, attempt int) time.Duration {
	exp := math.Pow(cfg.Multiplier, float64(attempt))
	raw := float64(cfg.Base) * exp
	if raw > float64(cfg.Cap) {
		raw = float64(cfg.Cap)
	}
	if cfg.JitterFull {
		raw = rand.Float64() * raw
	}
	return time.Duration(raw)
}
