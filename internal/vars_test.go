package internal

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("buildConfigOverrides", func() {
	AfterEach(func() {
		_ = os.Unsetenv("HELM_KUBECONTEXT")
	})

	It("returns empty CurrentContext when HELM_KUBECONTEXT is not set", func() {
		_ = os.Unsetenv("HELM_KUBECONTEXT")
		overrides := buildConfigOverrides()
		Expect(overrides.CurrentContext).To(BeEmpty())
	})

	It("sets CurrentContext from HELM_KUBECONTEXT", func() {
		_ = os.Setenv("HELM_KUBECONTEXT", "my-cluster")
		overrides := buildConfigOverrides()
		Expect(overrides.CurrentContext).To(Equal("my-cluster"))
	})

	It("updates CurrentContext when env var changes", func() {
		_ = os.Setenv("HELM_KUBECONTEXT", "cluster-a")
		Expect(buildConfigOverrides().CurrentContext).To(Equal("cluster-a"))

		_ = os.Setenv("HELM_KUBECONTEXT", "cluster-b")
		Expect(buildConfigOverrides().CurrentContext).To(Equal("cluster-b"))
	})
})
