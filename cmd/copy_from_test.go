package cmd

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseCopyFromMappings", func() {
	Context("valid mappings", func() {
		It("should parse a single mapping", func() {
			result, err := parseCopyFromMappings([]string{"/tmp/output.yaml:./output.yaml"})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKeyWithValue("/tmp/output.yaml", "./output.yaml"))
		})

		It("should parse multiple mappings", func() {
			result, err := parseCopyFromMappings([]string{
				"/tmp/a.txt:./a.txt",
				"/var/log/app.log:/home/user/app.log",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result).To(HaveKeyWithValue("/tmp/a.txt", "./a.txt"))
			Expect(result).To(HaveKeyWithValue("/var/log/app.log", "/home/user/app.log"))
		})

		It("should handle paths with multiple colons by splitting on first", func() {
			// SplitN with 2 means only first colon is the separator
			result, err := parseCopyFromMappings([]string{"/tmp/file:C:\\Users\\out"})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKeyWithValue("/tmp/file", "C:\\Users\\out"))
		})
	})

	Context("empty input", func() {
		It("should return empty map for empty slice", func() {
			result, err := parseCopyFromMappings([]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("should return empty map for nil slice", func() {
			result, err := parseCopyFromMappings(nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})
	})

	Context("invalid mappings", func() {
		It("should reject value without colon separator", func() {
			_, err := parseCopyFromMappings([]string{"/tmp/output.yaml"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid --copy-from format"))
		})

		It("should reject empty pod path", func() {
			_, err := parseCopyFromMappings([]string{":/host/path"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid --copy-from format"))
		})

		It("should reject empty host path", func() {
			_, err := parseCopyFromMappings([]string{"/pod/path:"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid --copy-from format"))
		})

		It("should reject completely empty value", func() {
			_, err := parseCopyFromMappings([]string{""})
			Expect(err).To(HaveOccurred())
		})

		It("should reject colon-only value", func() {
			_, err := parseCopyFromMappings([]string{":"})
			Expect(err).To(HaveOccurred())
		})
	})
})
