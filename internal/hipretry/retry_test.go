package hipretry

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("hipretry", func() {
	Describe("Retry (backward compat)", func() {
		It("returns nil on first success", func() {
			calls := 0
			err := Retry(3, func() error { calls++; return nil })
			Expect(err).NotTo(HaveOccurred())
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
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(2))
		})

		It("accumulates errors after all attempts fail", func() {
			calls := 0
			err := Retry(3, func() error {
				calls++
				return fmt.Errorf("fail %d", calls)
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("fail 1"))
			Expect(err.Error()).To(ContainSubstring("fail 2"))
			Expect(err.Error()).To(ContainSubstring("fail 3"))
		})

		It("does not call fn when maxAttempts=0", func() {
			calls := 0
			err := Retry(0, func() error { calls++; return fmt.Errorf("no") })
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(0))
		})
	})

	Describe("Permanent error fast-exit", func() {
		It("stops immediately on 404 NotFound", func() {
			calls := 0
			err := Retry(5, func() error {
				calls++
				return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusNotFound}}
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(Equal(1))
		})

		It("stops immediately on 403 Forbidden", func() {
			calls := 0
			err := Retry(5, func() error {
				calls++
				return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusForbidden}}
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(Equal(1))
		})

		It("stops immediately on 401 Unauthorized", func() {
			calls := 0
			err := Retry(5, func() error {
				calls++
				return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusUnauthorized}}
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(Equal(1))
		})

		It("stops immediately on 422 UnprocessableEntity", func() {
			calls := 0
			err := Retry(5, func() error {
				calls++
				return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusUnprocessableEntity}}
			})
			Expect(err).To(HaveOccurred())
			Expect(calls).To(Equal(1))
		})
	})

	Describe("Transient errors are retried", func() {
		It("retries on 429 TooManyRequests", func() {
			calls := 0
			err := Retry(3, func() error {
				calls++
				if calls < 3 {
					return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusTooManyRequests}}
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(3))
		})

		It("retries on 503 ServiceUnavailable", func() {
			calls := 0
			err := Retry(3, func() error {
				calls++
				if calls < 2 {
					return &apierrors.StatusError{ErrStatus: metav1.Status{Code: http.StatusServiceUnavailable}}
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(2))
		})

		It("retries on connection reset / EOF", func() {
			calls := 0
			err := Retry(3, func() error {
				calls++
				if calls < 2 {
					return stderrors.New("EOF")
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(2))
		})
	})

	Describe("Context cancellation during backoff sleep", func() {
		It("stops sleeping immediately when context is canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			calls := 0

			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			start := time.Now()
			err := RetryWithContext(ctx, 5, func() error {
				calls++
				return fmt.Errorf("always fail")
			})
			elapsed := time.Since(start)

			Expect(err).To(HaveOccurred())
			Expect(calls).To(BeNumerically("<=", 2)) // should not have done many attempts
			Expect(elapsed).To(BeNumerically("<", 200*time.Millisecond))
		})

		It("returns context.Canceled when pre-canceled", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := RetryWithContext(ctx, 3, func() error { return nil })
			Expect(stderrors.Is(err, context.Canceled)).To(BeTrue())
		})
	})

	Describe("Retry-After header respect", func() {
		It("uses Retry-After when larger than calculated backoff", func() {
			// This test is timing-based; we just verify it doesn't return too fast.
			start := time.Now()
			err := Retry(2, func() error {
				return &apierrors.StatusError{ErrStatus: metav1.Status{
					Code:    http.StatusTooManyRequests,
					Message: "Too many requests. Retry-After: 2",
				}}
			})
			elapsed := time.Since(start)
			Expect(err).To(HaveOccurred())
			// We expect at least ~1s sleep even with small base, because of the hint
			Expect(elapsed).To(BeNumerically(">=", 1500*time.Millisecond))
		})
	})

	Describe("Jitter math (statistical sanity)", func() {
		It("produces values within expected bounds for K8sAPIBackoff", func() {
			cfg := K8sAPIBackoff(0)
			for attempt := range 5 {
				d := calculateBackoff(cfg, attempt)
				Expect(d).To(BeNumerically(">=", 0))
				Expect(d).To(BeNumerically("<=", cfg.Cap))
			}
		})
	})

	Describe("Full jitter statistical verification", func() {
		It("produces well distributed jitter values over many iterations", func() {
			cfg := K8sAPIBackoff(0)
			const iterations = 500
			const attempt = 3 // base * 8 = 4s raw

			minSeen := time.Duration(1<<63 - 1)
			maxSeen := time.Duration(0)

			for range iterations {
				d := calculateBackoff(cfg, attempt)
				if d < minSeen {
					minSeen = d
				}
				if d > maxSeen {
					maxSeen = d
				}
			}

			// With full jitter we should see values near 0 and near the theoretical max for this attempt
			Expect(minSeen).To(BeNumerically("<=", 50*time.Millisecond))
			Expect(maxSeen).To(BeNumerically(">=", 3500*time.Millisecond)) // close to 4s raw
		})
	})

	Describe("Retry-After stronger verification", func() {
		It("honors Retry-After even when calculated backoff is smaller", func() {
			start := time.Now()
			_ = Retry(2, func() error {
				return &apierrors.StatusError{ErrStatus: metav1.Status{
					Code:    http.StatusTooManyRequests,
					Message: "retry later. Retry-After: 1",
				}}
			})
			elapsed := time.Since(start)
			// We expect at least ~900ms of sleep because of the hint
			Expect(elapsed).To(BeNumerically(">=", 850*time.Millisecond))
		})
	})

	Describe("Unlimited retries mode (MaxAttempts=0)", func() {
		It("never calls fn when maxAttempts=0", func() {
			calls := 0
			err := RetryWithContext(context.Background(), 0, func() error {
				calls++
				return fmt.Errorf("should not run")
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal(0))
		})
	})

	Describe("extractRetryAfter", func() {
		It("extracts seconds from ErrStatus.Details.Causes with Type=RetryAfter", func() {
			err := &apierrors.StatusError{ErrStatus: metav1.Status{
				Details: &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{Type: "RetryAfter", Message: "5"},
					},
				},
			}}
			Expect(extractRetryAfter(err)).To(Equal(5 * time.Second))
		})
	})

	Describe("Context deadline during backoff", func() {
		It("stops immediately when deadline is exceeded", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			defer cancel()

			start := time.Now()
			err := RetryWithContext(ctx, 10, func() error {
				return fmt.Errorf("always fail")
			})
			elapsed := time.Since(start)

			Expect(err).To(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 300*time.Millisecond))
		})
	})
})
