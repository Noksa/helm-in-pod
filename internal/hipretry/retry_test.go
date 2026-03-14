package hipretry

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Retry", func() {
	It("should return nil when function succeeds on first attempt", func() {
		calls := 0
		err := Retry(3, func() error {
			calls++
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("should retry and succeed on second attempt", func() {
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

	It("should return combined errors after all attempts fail", func() {
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

	It("should call function exactly once when maxAttempts is 1", func() {
		calls := 0
		err := Retry(1, func() error {
			calls++
			return fmt.Errorf("always fail")
		})
		Expect(err).To(HaveOccurred())
		Expect(calls).To(Equal(1))
	})

	It("should not call function when maxAttempts is 0", func() {
		calls := 0
		err := Retry(0, func() error {
			calls++
			return fmt.Errorf("should not run")
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(0))
	})

	It("should succeed on the last attempt", func() {
		calls := 0
		err := Retry(5, func() error {
			calls++
			if calls < 5 {
				return fmt.Errorf("fail %d", calls)
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(5))
	})
})
