package cmd

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("getDaemonName", func() {
	var originalEnv string

	BeforeEach(func() {
		originalEnv = os.Getenv(hipconsts.EnvDaemonName)
	})

	AfterEach(func() {
		if originalEnv != "" {
			_ = os.Setenv(hipconsts.EnvDaemonName, originalEnv)
		} else {
			_ = os.Unsetenv(hipconsts.EnvDaemonName)
		}
	})

	Context("when name is provided directly", func() {
		It("should return the provided name", func() {
			name, err := getDaemonName("my-daemon")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("my-daemon"))
		})

		It("should prefer provided name over environment variable", func() {
			_ = os.Setenv(hipconsts.EnvDaemonName, "env-daemon")
			name, err := getDaemonName("my-daemon")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("my-daemon"))
		})
	})

	Context("when name is not provided", func() {
		It("should use environment variable", func() {
			_ = os.Setenv(hipconsts.EnvDaemonName, "env-daemon")
			name, err := getDaemonName("")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("env-daemon"))
		})

		It("should fail when neither name nor env var is set", func() {
			_ = os.Unsetenv(hipconsts.EnvDaemonName)
			name, err := getDaemonName("")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("--name is required"))
			Expect(err.Error()).To(ContainSubstring(hipconsts.EnvDaemonName))
			Expect(name).To(BeEmpty())
		})
	})

	Context("edge cases", func() {
		It("should handle whitespace-only name as empty", func() {
			_ = os.Unsetenv(hipconsts.EnvDaemonName)
			name, err := getDaemonName("   ")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("   "))
		})

		It("should handle special characters in name", func() {
			name, err := getDaemonName("my-daemon-123_test")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("my-daemon-123_test"))
		})
	})
})
