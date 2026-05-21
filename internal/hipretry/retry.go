package hipretry

import (
	"context"
	stderrors "errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
)

// Func is the retryable function signature.
type Func func() error

// BackoffConfig defines exponential backoff + jitter behavior.
type BackoffConfig struct {
	Base       time.Duration // initial backoff (e.g. 500*time.Millisecond)
	Multiplier float64       // growth factor (e.g. 2.0)
	Cap        time.Duration // maximum backoff (e.g. 60*time.Second)
	JitterFull bool          // true = full jitter (rand[0, min(cap, base*multiplier^attempt)])
}

// DefaultBackoff is the conservative preset (no jitter) used by Retry / RetryWithContext.
var DefaultBackoff = BackoffConfig{
	Base:       time.Second,
	Multiplier: 2.0,
	Cap:        60 * time.Second,
	JitterFull: false,
}

// K8sAPIBackoff returns a jittered config tuned for Kubernetes API calls.
// It uses a lower base and full jitter to avoid thundering herd.
func K8sAPIBackoff(attempts int) BackoffConfig {
	// attempts is currently unused but kept for future tuning / validation
	_ = attempts
	return BackoffConfig{
		Base:       500 * time.Millisecond,
		Multiplier: 2.0,
		Cap:        60 * time.Second,
		JitterFull: true,
	}
}

// isPermanentError returns true for errors that should never be retried.
func isPermanentError(err error) bool {
	// Kubernetes API permanent errors
	if apiErr, ok := stderrors.AsType[*apierrors.StatusError](err); ok {
		code := apiErr.ErrStatus.Code
		switch code {
		case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized, http.StatusUnprocessableEntity:
			return true
		}
	}

	// Also treat wrapped meta.NoKindMatchError and similar as permanent
	if meta.IsNoMatchError(err) {
		return true
	}

	return false
}

// extractRetryAfter returns the server-suggested delay from a 429/503 response.
func extractRetryAfter(err error) time.Duration {
	if apiErr, ok := stderrors.AsType[*apierrors.StatusError](err); ok {
		if apiErr.ErrStatus.Details != nil {
			for _, cause := range apiErr.ErrStatus.Details.Causes {
				if cause.Type == "RetryAfter" || strings.EqualFold(cause.Message, "retry-after") {
					if secs, parseErr := strconv.Atoi(cause.Message); parseErr == nil && secs > 0 {
						return time.Duration(secs) * time.Second
					}
				}
			}
		}
	}

	// Fallback: look for "Retry-After: 5" in the error string (some clients embed it)
	msg := err.Error()
	if _, after, ok := strings.Cut(msg, "Retry-After:"); ok {
		rest := strings.TrimSpace(after)
		if secs, err := strconv.Atoi(strings.Fields(rest)[0]); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return 0
}

// isTransientError returns true for errors worth retrying.
func isTransientError(err error) bool {
	if isPermanentError(err) {
		return false
	}

	// Common transient network / server errors
	// Note: context.Canceled is NOT transient — it indicates intentional cancellation.
	if stderrors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "EOF") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "i/o timeout") {
		return true
	}

	// Kubernetes 429 / 503 / 500 (etcd hints)
	if apiErr, ok := stderrors.AsType[*apierrors.StatusError](err); ok {
		code := apiErr.ErrStatus.Code
		if code == http.StatusTooManyRequests ||
			code == http.StatusServiceUnavailable ||
			code == http.StatusInternalServerError {
			return true
		}
	}

	// Rest client request errors that are usually transient
	if _, ok := stderrors.AsType[*rest.RequestConstructionError](err); ok {
		return true
	}

	return false
}

// calculateBackoff computes the next sleep duration according to the config.
func calculateBackoff(cfg BackoffConfig, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	raw := float64(cfg.Base) * pow(cfg.Multiplier, attempt)
	if raw > float64(cfg.Cap) {
		raw = float64(cfg.Cap)
	}
	if cfg.JitterFull {
		// Full jitter: random value in [0, min(cap, base * multiplier^attempt)]
		jittered := min(time.Duration(rand.Float64()*raw), cfg.Cap)
		return jittered
	}
	return time.Duration(raw)
}

func pow(base float64, exp int) float64 {
	res := 1.0
	for range exp {
		res *= base
	}
	return res
}

// Retry is the backward-compatible entry point (uses DefaultBackoff, no context).
func Retry(maxAttempts int, fn Func) error {
	return RetryWithBackoff(context.Background(), DefaultBackoff, maxAttempts, fn)
}

// RetryWithContext is a drop-in replacement that respects context cancellation
// during the backoff sleep.
func RetryWithContext(ctx context.Context, maxAttempts int, fn Func) error {
	return RetryWithBackoff(ctx, DefaultBackoff, maxAttempts, fn)
}

// RetryWithBackoff is the full-featured entry point.
func RetryWithBackoff(ctx context.Context, cfg BackoffConfig, maxAttempts int, fn Func) error {
	if maxAttempts == 0 {
		return nil
	}
	if maxAttempts < 0 {
		maxAttempts = 0
	}

	var mErr error
	for attempt := range maxAttempts {
		// Check for cancellation before calling fn
		if ctx.Err() != nil {
			mErr = multierr.Append(mErr, ctx.Err())
			break
		}

		err := fn()
		if err == nil {
			return nil
		}

		mErr = multierr.Append(mErr, err)

		// Permanent error → stop immediately
		if isPermanentError(err) {
			break
		}

		// Last attempt → don't sleep
		if attempt == maxAttempts-1 {
			break
		}

		// Compute sleep duration
		sleep := calculateBackoff(cfg, attempt)

		// Respect Retry-After header if present and larger
		if retryAfter := extractRetryAfter(err); retryAfter > sleep {
			sleep = retryAfter
		}

		// Sleep with context awareness
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			mErr = multierr.Append(mErr, ctx.Err())
			return mErr
		case <-timer.C:
		}
	}
	return mErr
}

// IsPermanentError exposes the classifier for tests / external use.
func IsPermanentError(err error) bool { return isPermanentError(err) }

// IsTransientError exposes the classifier for tests / external use.
func IsTransientError(err error) bool { return isTransientError(err) }
