package cmd

import (
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("validateResourceFlags", func() {
	var (
		cmd  *cobra.Command
		opts *cmdoptions.ExecOptions
	)

	BeforeEach(func() {
		opts = &cmdoptions.ExecOptions{}
		cmd = &cobra.Command{}
		addPodCreationFlags(cmd, opts)
	})

	Context("when no flags are set", func() {
		It("should use default values", func() {
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("1100m"))
			Expect(opts.CpuLimit).To(Equal("1100m"))
			Expect(opts.MemoryRequest).To(Equal("500Mi"))
			Expect(opts.MemoryLimit).To(Equal("500Mi"))
		})
	})

	Context("when using deprecated flags", func() {
		It("should set both request and limit from --cpu", func() {
			_ = cmd.Flags().Set("cpu", "500m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Cpu).To(Equal("500m"))
			Expect(opts.CpuRequest).To(Equal("500m"))
			Expect(opts.CpuLimit).To(Equal("500m"))
			Expect(opts.MemoryRequest).To(Equal("500Mi"))
			Expect(opts.MemoryLimit).To(Equal("500Mi"))
		})

		It("should set both request and limit from --memory", func() {
			_ = cmd.Flags().Set("memory", "1Gi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("1100m"))
			Expect(opts.CpuLimit).To(Equal("1100m"))
			Expect(opts.Memory).To(Equal("1Gi"))
			Expect(opts.MemoryRequest).To(Equal("1Gi"))
			Expect(opts.MemoryLimit).To(Equal("1Gi"))
		})

		It("should handle both deprecated flags together", func() {
			_ = cmd.Flags().Set("cpu", "2000m")
			_ = cmd.Flags().Set("memory", "2Gi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Cpu).To(Equal("2000m"))
			Expect(opts.CpuRequest).To(Equal("2000m"))
			Expect(opts.CpuLimit).To(Equal("2000m"))
			Expect(opts.Memory).To(Equal("2Gi"))
			Expect(opts.MemoryRequest).To(Equal("2Gi"))
			Expect(opts.MemoryLimit).To(Equal("2Gi"))
		})
	})

	Context("when using new CPU flags", func() {
		It("should set only request when --cpu-request is specified", func() {
			_ = cmd.Flags().Set("cpu-request", "250m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("250m"))
			Expect(opts.CpuLimit).To(BeEmpty())
			Expect(opts.MemoryRequest).To(Equal("500Mi"))
			Expect(opts.MemoryLimit).To(Equal("500Mi"))
		})

		It("should set only limit when --cpu-limit is specified", func() {
			_ = cmd.Flags().Set("cpu-limit", "2000m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(BeEmpty())
			Expect(opts.CpuLimit).To(Equal("2000m"))
			Expect(opts.MemoryRequest).To(Equal("500Mi"))
			Expect(opts.MemoryLimit).To(Equal("500Mi"))
		})

		It("should set both request and limit when both are specified", func() {
			_ = cmd.Flags().Set("cpu-request", "250m")
			_ = cmd.Flags().Set("cpu-limit", "1000m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("250m"))
			Expect(opts.CpuLimit).To(Equal("1000m"))
			Expect(opts.MemoryRequest).To(Equal("500Mi"))
			Expect(opts.MemoryLimit).To(Equal("500Mi"))
		})
	})

	Context("when using new memory flags", func() {
		It("should set only request when --memory-request is specified", func() {
			_ = cmd.Flags().Set("memory-request", "128Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("1100m"))
			Expect(opts.CpuLimit).To(Equal("1100m"))
			Expect(opts.MemoryRequest).To(Equal("128Mi"))
			Expect(opts.MemoryLimit).To(BeEmpty())
		})

		It("should set only limit when --memory-limit is specified", func() {
			_ = cmd.Flags().Set("memory-limit", "1Gi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("1100m"))
			Expect(opts.CpuLimit).To(Equal("1100m"))
			Expect(opts.MemoryRequest).To(BeEmpty())
			Expect(opts.MemoryLimit).To(Equal("1Gi"))
		})

		It("should set both request and limit when both are specified", func() {
			_ = cmd.Flags().Set("memory-request", "256Mi")
			_ = cmd.Flags().Set("memory-limit", "512Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("1100m"))
			Expect(opts.CpuLimit).To(Equal("1100m"))
			Expect(opts.MemoryRequest).To(Equal("256Mi"))
			Expect(opts.MemoryLimit).To(Equal("512Mi"))
		})
	})

	Context("when using all new flags together", func() {
		It("should set all values correctly", func() {
			_ = cmd.Flags().Set("cpu-request", "250m")
			_ = cmd.Flags().Set("cpu-limit", "1000m")
			_ = cmd.Flags().Set("memory-request", "256Mi")
			_ = cmd.Flags().Set("memory-limit", "512Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("250m"))
			Expect(opts.CpuLimit).To(Equal("1000m"))
			Expect(opts.MemoryRequest).To(Equal("256Mi"))
			Expect(opts.MemoryLimit).To(Equal("512Mi"))
		})
	})

	Context("when mixing deprecated and new flags", func() {
		It("should allow deprecated cpu with new memory flags", func() {
			_ = cmd.Flags().Set("cpu", "500m")
			_ = cmd.Flags().Set("memory-request", "256Mi")
			_ = cmd.Flags().Set("memory-limit", "512Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.Cpu).To(Equal("500m"))
			Expect(opts.CpuRequest).To(Equal("500m"))
			Expect(opts.CpuLimit).To(Equal("500m"))
			Expect(opts.MemoryRequest).To(Equal("256Mi"))
			Expect(opts.MemoryLimit).To(Equal("512Mi"))
		})

		It("should allow deprecated memory with new cpu flags", func() {
			_ = cmd.Flags().Set("memory", "1Gi")
			_ = cmd.Flags().Set("cpu-request", "250m")
			_ = cmd.Flags().Set("cpu-limit", "1000m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("250m"))
			Expect(opts.CpuLimit).To(Equal("1000m"))
			Expect(opts.Memory).To(Equal("1Gi"))
			Expect(opts.MemoryRequest).To(Equal("1Gi"))
			Expect(opts.MemoryLimit).To(Equal("1Gi"))
		})
	})

	Context("when using conflicting CPU flags", func() {
		It("should fail when --cpu and --cpu-request are used together", func() {
			_ = cmd.Flags().Set("cpu", "500m")
			_ = cmd.Flags().Set("cpu-request", "250m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --cpu with --cpu-request or --cpu-limit"))
		})

		It("should fail when --cpu and --cpu-limit are used together", func() {
			_ = cmd.Flags().Set("cpu", "500m")
			_ = cmd.Flags().Set("cpu-limit", "1000m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --cpu with --cpu-request or --cpu-limit"))
		})

		It("should fail when --cpu is used with both new cpu flags", func() {
			_ = cmd.Flags().Set("cpu", "500m")
			_ = cmd.Flags().Set("cpu-request", "250m")
			_ = cmd.Flags().Set("cpu-limit", "1000m")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --cpu with --cpu-request or --cpu-limit"))
		})
	})

	Context("when using conflicting memory flags", func() {
		It("should fail when --memory and --memory-request are used together", func() {
			_ = cmd.Flags().Set("memory", "1Gi")
			_ = cmd.Flags().Set("memory-request", "256Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --memory with --memory-request or --memory-limit"))
		})

		It("should fail when --memory and --memory-limit are used together", func() {
			_ = cmd.Flags().Set("memory", "1Gi")
			_ = cmd.Flags().Set("memory-limit", "512Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --memory with --memory-request or --memory-limit"))
		})

		It("should fail when --memory is used with both new memory flags", func() {
			_ = cmd.Flags().Set("memory", "1Gi")
			_ = cmd.Flags().Set("memory-request", "256Mi")
			_ = cmd.Flags().Set("memory-limit", "512Mi")
			err := validateResourceFlags(cmd, opts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot use --memory with --memory-request or --memory-limit"))
		})
	})

	Context("edge cases", func() {
		It("should handle empty string values", func() {
			_ = cmd.Flags().Set("cpu-request", "")
			_ = cmd.Flags().Set("memory-request", "")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(BeEmpty())
			Expect(opts.CpuLimit).To(BeEmpty())
			Expect(opts.MemoryRequest).To(BeEmpty())
			Expect(opts.MemoryLimit).To(BeEmpty())
		})

		It("should handle zero values", func() {
			_ = cmd.Flags().Set("cpu-request", "0")
			_ = cmd.Flags().Set("memory-request", "0")
			err := validateResourceFlags(cmd, opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(opts.CpuRequest).To(Equal("0"))
			Expect(opts.CpuLimit).To(BeEmpty())
			Expect(opts.MemoryRequest).To(Equal("0"))
			Expect(opts.MemoryLimit).To(BeEmpty())
		})
	})
})
