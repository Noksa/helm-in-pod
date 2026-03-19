package cmd

import (
	"os/user"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("expand", func() {
	It("should return the path unchanged when it does not start with ~", func() {
		result, err := expand("/some/absolute/path")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("/some/absolute/path"))
	})

	It("should return empty string unchanged", func() {
		result, err := expand("")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("should expand ~ to the user home directory", func() {
		usr, err := user.Current()
		Expect(err).NotTo(HaveOccurred())

		result, err := expand("~/Documents")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(usr.HomeDir + "/Documents"))
	})

	It("should expand bare ~ to the user home directory", func() {
		usr, err := user.Current()
		Expect(err).NotTo(HaveOccurred())

		result, err := expand("~")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(usr.HomeDir))
	})

	It("should handle relative paths without tilde", func() {
		result, err := expand("relative/path")
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("relative/path"))
	})
})
