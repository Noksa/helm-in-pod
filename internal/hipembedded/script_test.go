package hipembedded

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetShScript", func() {
	It("should return a non-empty script", func() {
		script := GetShScript()
		Expect(script).NotTo(BeEmpty())
	})

	It("should contain a shebang line", func() {
		script := GetShScript()
		Expect(script).To(HavePrefix("#!/"))
	})

	It("should contain the trap function", func() {
		script := GetShScript()
		Expect(script).To(ContainSubstring("trapMe"))
	})
})
