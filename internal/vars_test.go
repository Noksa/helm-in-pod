package internal

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// applyNamespaceOverride mirrors the env-var branch inside InitManagers so we can
// test it without needing a live Kubernetes cluster.
func applyNamespaceOverride() {
	if ns := os.Getenv(hipconsts.EnvNamespace); ns != "" {
		hipconsts.Namespace = ns
	}
}

var _ = Describe("HELM_IN_POD_NAMESPACE env var", func() {
	var originalNamespace string

	BeforeEach(func() {
		originalNamespace = hipconsts.Namespace
	})
	AfterEach(func() {
		hipconsts.Namespace = originalNamespace
		_ = os.Unsetenv(hipconsts.EnvNamespace)
	})

	It("leaves hipconsts.Namespace at its default when env var is not set", func() {
		_ = os.Unsetenv(hipconsts.EnvNamespace)
		applyNamespaceOverride()
		Expect(hipconsts.Namespace).To(Equal(originalNamespace))
	})

	It("sets hipconsts.Namespace from HELM_IN_POD_NAMESPACE", func() {
		_ = os.Setenv(hipconsts.EnvNamespace, "my-custom-ns")
		applyNamespaceOverride()
		Expect(hipconsts.Namespace).To(Equal("my-custom-ns"))
	})

	It("updates hipconsts.Namespace when env var changes between calls", func() {
		_ = os.Setenv(hipconsts.EnvNamespace, "ns-alpha")
		applyNamespaceOverride()
		Expect(hipconsts.Namespace).To(Equal("ns-alpha"))

		hipconsts.Namespace = originalNamespace
		_ = os.Setenv(hipconsts.EnvNamespace, "ns-beta")
		applyNamespaceOverride()
		Expect(hipconsts.Namespace).To(Equal("ns-beta"))
	})

	It("ignores an empty HELM_IN_POD_NAMESPACE and keeps the existing value", func() {
		_ = os.Setenv(hipconsts.EnvNamespace, "")
		applyNamespaceOverride()
		Expect(hipconsts.Namespace).To(Equal(originalNamespace))
	})
})

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
