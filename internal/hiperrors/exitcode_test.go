package hiperrors

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHiperrors(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hiperrors Suite")
}

var _ = Describe("ExitCodeError", func() {
	It("should format the error message with the exit code", func() {
		err := &ExitCodeError{Code: 2}
		Expect(err.Error()).To(Equal("command exited with code 2"))
	})

	It("should be detectable via errors.As", func() {
		err := fmt.Errorf("wrapped: %w", &ExitCodeError{Code: 42})
		var exitErr *ExitCodeError
		Expect(errors.As(err, &exitErr)).To(BeTrue())
		Expect(exitErr.Code).To(Equal(int32(42)))
	})

	It("should not match errors.As for unrelated errors", func() {
		err := fmt.Errorf("some other error")
		var exitErr *ExitCodeError
		Expect(errors.As(err, &exitErr)).To(BeFalse())
	})

	It("should preserve exit code 0", func() {
		err := &ExitCodeError{Code: 0}
		Expect(err.Error()).To(Equal("command exited with code 0"))
	})
})
