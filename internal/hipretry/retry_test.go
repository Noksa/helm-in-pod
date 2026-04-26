package hipretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var gr = schema.GroupResource{Resource: "pods"}

// ─── Retry (backward-compat) ──────────────────────────────────────────────────

var _ = Describe("Retry (backward-compat)", func() {
	It("returns nil on first success", func() {
		calls := 0
		Expect(Retry(3, func() error { calls++; return nil })).To(Succeed())
		Expect(calls).To(Equal(1))
	})

	It("retries and succeeds on second attempt", func() {
		calls := 0
		err := Retry(3, func() error {
			calls++
			if calls < 2 {
				return fmt.Errorf("fail %d", calls)
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("returns combined errors after all attempts", func() {
		calls := 0
		err := Retry(3, func() error { calls++; return fmt.Errorf("fail %d", calls) })
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(3))
		Expect(err.Error()).To(ContainSubstring("fail 1"))
		Expect(err.Error()).To(ContainSubstring("fail 2"))
		Expect(err.Error()).To(ContainSubstring("fail 3"))
	})

	It("calls function exactly once when maxAttempts is 1", func() {
		calls := 0
		Expect(Retry(1, func() error { calls++; return fmt.Errorf("x") })).To(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("does not call function when maxAttempts is 0 (backward-compat)", func() {
		calls := 0
		Expect(Retry(0, func() error { calls++; return fmt.Errorf("x") })).To(Succeed())
		Expect(calls).To(Equal(0))
	})

	It("succeeds on last attempt", func() {
		calls := 0
		err := Retry(5, func() error {
			calls++
			if calls < 5 {
				return fmt.Errorf("fail %d", calls)
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(5))
	})
})

// ─── RetryWithContext ─────────────────────────────────────────────────────────

var _ = Describe("RetryWithContext", func() {
	It("behaves identically to Retry when context is not canceled", func() {
		calls := 0
		err := RetryWithContext(context.Background(), 3, func() error {
			calls++
			if calls < 2 {
				return fmt.Errorf("transient")
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("does not call fn when maxAttempts is 0", func() {
		calls := 0
		Expect(RetryWithContext(context.Background(), 0, func() error {
			calls++
			return fmt.Errorf("x")
		})).To(Succeed())
		Expect(calls).To(Equal(0))
	})

	It("stops the backoff sleep immediately when context is canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		// Large sleep config so the test would stall without context support.
		slowCfg := BackoffConfig{
			Base:        500 * time.Millisecond,
			Cap:         10 * time.Second,
			Multiplier:  2.0,
			JitterFull:  false,
			MaxAttempts: 50,
		}
		err := RetryWithBackoff(ctx, slowCfg, func() error {
			calls++
			return fmt.Errorf("keep failing")
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(BeNumerically("<", 50))
	})
})

// ─── RetryWithBackoff ─────────────────────────────────────────────────────────

var _ = Describe("RetryWithBackoff", func() {
	// Ultra-fast config for unit tests.
	fast := BackoffConfig{
		Base:        time.Millisecond,
		Cap:         5 * time.Millisecond,
		Multiplier:  2.0,
		JitterFull:  false,
		MaxAttempts: 5,
	}

	It("succeeds immediately on first try", func() {
		calls := 0
		Expect(RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return nil
		})).To(Succeed())
		Expect(calls).To(Equal(1))
	})

	It("retries transient io.EOF and succeeds on 3rd attempt", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 3 {
				return io.EOF
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(3))
	})

	It("stops after exactly MaxAttempts on persistent transient error", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return io.EOF
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(fast.MaxAttempts))
	})

	It("fast-exits on K8s 404 NotFound (permanent)", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return k8serrors.NewNotFound(gr, "my-pod")
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(1), "404 is permanent – must not retry")
	})

	It("fast-exits on K8s 403 Forbidden (permanent)", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return k8serrors.NewForbidden(gr, "obj", errors.New("no access"))
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("fast-exits on K8s 401 Unauthorized (permanent)", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return k8serrors.NewUnauthorized("bad token")
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("fast-exits on context.Canceled (permanent)", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return context.Canceled
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("retries K8s 429 TooManyRequests", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 3 {
				return k8serrors.NewTooManyRequests("slow down", 1)
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(3))
	})

	It("retries K8s 503 ServiceUnavailable", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 2 {
				return k8serrors.NewServiceUnavailable("overloaded")
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("retries K8s 409 Conflict (not permanent)", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 2 {
				return k8serrors.NewConflict(gr, "obj", errors.New("version mismatch"))
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("retries K8s 500 with etcd hint", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 2 {
				return k8serrors.NewInternalError(errors.New("etcd cluster is unavailable"))
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("exhausts all attempts on K8s 500 without transient hint", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			return k8serrors.NewInternalError(errors.New("unknown internal panic"))
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(fast.MaxAttempts))
	})

	It("retries syscall.ECONNRESET", func() {
		calls := 0
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 3 {
				return syscall.ECONNRESET
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(3))
	})

	It("retries url.Error wrapping io.EOF", func() {
		calls := 0
		wrapped := &url.Error{Op: "Post", URL: "https://k8s-api:6443", Err: io.EOF}
		err := RetryWithBackoff(context.Background(), fast, func() error {
			calls++
			if calls < 2 {
				return wrapped
			}
			return nil
		})
		Expect(err).To(Succeed())
		Expect(calls).To(Equal(2))
	})

	It("respects context cancellation mid-retry", func() {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0

		slowCfg := BackoffConfig{
			Base:        200 * time.Millisecond,
			Cap:         1 * time.Second,
			Multiplier:  2.0,
			JitterFull:  false,
			MaxAttempts: 20,
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := RetryWithBackoff(ctx, slowCfg, func() error {
			calls++
			return io.EOF
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(BeNumerically("<", 20))
	})

	It("uses Retry-After hint when it exceeds the calculated backoff", func() {
		// Calculated sleep on attempt 0 with this config is 1ms; the server
		// hint is 200ms, so the actual sleep must be at least the server hint.
		cfg := BackoffConfig{
			Base:        time.Millisecond,
			Cap:         time.Second,
			Multiplier:  2.0,
			MaxAttempts: 2,
		}

		retryAfter := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    429,
				Reason:  metav1.StatusReasonTooManyRequests,
				Details: &metav1.StatusDetails{RetryAfterSeconds: 1},
			},
		}

		start := time.Now()
		err := RetryWithBackoff(context.Background(), cfg, func() error {
			return retryAfter
		})
		elapsed := time.Since(start)

		Expect(err).To(HaveOccurred())
		// Sleep must respect the 1s server hint, not the 1ms calculated value.
		Expect(elapsed).To(BeNumerically(">=", 1*time.Second))
	})

	Context("with MaxAttempts=0 (unlimited)", func() {
		It("succeeds without exhausting attempts when fn eventually returns nil", func() {
			unlimited := BackoffConfig{
				Base:        time.Millisecond,
				Cap:         5 * time.Millisecond,
				Multiplier:  2.0,
				MaxAttempts: 0,
			}
			calls := 0
			err := RetryWithBackoff(context.Background(), unlimited, func() error {
				calls++
				if calls < 5 {
					return io.EOF
				}
				return nil
			})
			Expect(err).To(Succeed())
			Expect(calls).To(Equal(5))
		})

		It("stops only when context deadline expires", func() {
			unlimited := BackoffConfig{
				Base:        2 * time.Millisecond,
				Cap:         5 * time.Millisecond,
				Multiplier:  2.0,
				MaxAttempts: 0,
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			calls := 0
			err := RetryWithBackoff(ctx, unlimited, func() error {
				calls++
				return io.EOF
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(BeNumerically(">", 1))
		})

		It("returns immediately when ctx is already canceled", func() {
			unlimited := BackoffConfig{
				Base:        time.Millisecond,
				Cap:         5 * time.Millisecond,
				Multiplier:  2.0,
				MaxAttempts: 0,
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			calls := 0
			err := RetryWithBackoff(ctx, unlimited, func() error {
				calls++
				return io.EOF
			})
			Expect(err).To(MatchError(context.Canceled))
			Expect(calls).To(Equal(0))
		})
	})
})

// ─── retryAfterSeconds ────────────────────────────────────────────────────────

var _ = Describe("retryAfterSeconds", func() {
	It("returns 0 for non-status errors", func() {
		Expect(retryAfterSeconds(fmt.Errorf("plain error"))).To(Equal(time.Duration(0)))
	})

	It("returns 0 for status errors without RetryAfterSeconds", func() {
		err := k8serrors.NewServiceUnavailable("overloaded")
		Expect(retryAfterSeconds(err)).To(Equal(time.Duration(0)))
	})

	It("extracts duration from 429 with RetryAfterSeconds", func() {
		err := &k8serrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Code:    429,
				Reason:  metav1.StatusReasonTooManyRequests,
				Details: &metav1.StatusDetails{RetryAfterSeconds: 5},
			},
		}
		Expect(retryAfterSeconds(err)).To(Equal(5 * time.Second))
	})

	It("returns 0 for nil", func() {
		Expect(retryAfterSeconds(nil)).To(Equal(time.Duration(0)))
	})
})

// ─── nextSleep (math) ─────────────────────────────────────────────────────────

var _ = Describe("nextSleep", func() {
	cfg := BackoffConfig{Base: time.Second, Cap: 10 * time.Second, Multiplier: 2.0}

	It("returns Base on attempt 0", func() {
		Expect(nextSleep(cfg, 0)).To(Equal(time.Second))
	})
	It("doubles on attempt 1", func() {
		Expect(nextSleep(cfg, 1)).To(Equal(2 * time.Second))
	})
	It("quadruples on attempt 2", func() {
		Expect(nextSleep(cfg, 2)).To(Equal(4 * time.Second))
	})
	It("is capped at Cap on high attempt", func() {
		Expect(nextSleep(cfg, 20)).To(Equal(10 * time.Second))
	})

	It("full-jitter always stays within [0, Cap]", func() {
		jCfg := cfg
		jCfg.JitterFull = true
		for range 500 {
			d := nextSleep(jCfg, 10)
			Expect(d).To(BeNumerically(">=", 0))
			Expect(d).To(BeNumerically("<=", cfg.Cap))
		}
	})
})

// ─── isPermanent ──────────────────────────────────────────────────────────────

var _ = Describe("isPermanent", func() {
	DescribeTable("permanent → true",
		func(err error) { Expect(isPermanent(err)).To(BeTrue()) },
		Entry("404 NotFound", k8serrors.NewNotFound(gr, "x")),
		Entry("403 Forbidden", k8serrors.NewForbidden(gr, "x", errors.New("no"))),
		Entry("401 Unauthorized", k8serrors.NewUnauthorized("bad")),
		Entry("422 Invalid", k8serrors.NewInvalid(schema.GroupKind{}, "x", nil)),
		Entry("context.Canceled", context.Canceled),
	)

	DescribeTable("non-permanent → false",
		func(err error) { Expect(isPermanent(err)).To(BeFalse()) },
		Entry("nil", nil),
		Entry("429 TooManyRequests", k8serrors.NewTooManyRequests("slow", 5)),
		Entry("503 ServiceUnavailable", k8serrors.NewServiceUnavailable("down")),
		Entry("409 Conflict", k8serrors.NewConflict(gr, "x", errors.New("version"))),
		Entry("io.EOF", io.EOF),
		Entry("ECONNRESET", syscall.ECONNRESET),
		Entry("generic error", errors.New("oops")),
		Entry("context.DeadlineExceeded", context.DeadlineExceeded),
	)
})

// ─── backoff presets ──────────────────────────────────────────────────────────

var _ = Describe("backoff presets", func() {
	It("DefaultBackoff returns expected values", func() {
		cfg := DefaultBackoff()
		Expect(cfg.Base).To(Equal(1 * time.Second))
		Expect(cfg.Cap).To(Equal(30 * time.Second))
		Expect(cfg.Multiplier).To(Equal(2.0))
		Expect(cfg.JitterFull).To(BeFalse())
		Expect(cfg.MaxAttempts).To(Equal(0))
	})

	It("K8sAPIBackoff returns expected values with full jitter", func() {
		cfg := K8sAPIBackoff(5)
		Expect(cfg.Base).To(Equal(500 * time.Millisecond))
		Expect(cfg.Cap).To(Equal(30 * time.Second))
		Expect(cfg.Multiplier).To(Equal(2.0))
		Expect(cfg.JitterFull).To(BeTrue())
		Expect(cfg.MaxAttempts).To(Equal(5))
	})

	It("K8sAPIBackoff with jitter retries transient errors and respects context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()
		err := RetryWithBackoff(ctx, K8sAPIBackoff(50), func() error {
			calls++
			return fmt.Errorf("transient")
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(BeNumerically("<", 50))
	})
})
