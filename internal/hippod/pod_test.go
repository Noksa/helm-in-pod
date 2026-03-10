package hippod

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("parseToleration", func() {
	Context("when parsing valid toleration strings", func() {
		It("should tolerate all taints", func() {
			got, err := parseToleration("::Exists")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Operator).To(Equal(corev1.TolerationOpExists))
			Expect(got.Key).To(BeEmpty())
			Expect(got.Value).To(BeEmpty())
			Expect(got.Effect).To(BeEmpty())
		})

		It("should parse key with any effect", func() {
			got, err := parseToleration("node.kubernetes.io/disk-pressure=::Exists")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Key).To(Equal("node.kubernetes.io/disk-pressure"))
			Expect(got.Operator).To(Equal(corev1.TolerationOpExists))
		})

		It("should parse key with any value for specific effect", func() {
			got, err := parseToleration("node.kubernetes.io/not-ready=:NoExecute:Exists")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Key).To(Equal("node.kubernetes.io/not-ready"))
			Expect(got.Effect).To(Equal(corev1.TaintEffectNoExecute))
			Expect(got.Operator).To(Equal(corev1.TolerationOpExists))
		})

		It("should parse key=value with specific effect", func() {
			got, err := parseToleration("dedicated=gpu:NoSchedule:Equal")
			Expect(err).NotTo(HaveOccurred())
			Expect(got.Key).To(Equal("dedicated"))
			Expect(got.Value).To(Equal("gpu"))
			Expect(got.Effect).To(Equal(corev1.TaintEffectNoSchedule))
			Expect(got.Operator).To(Equal(corev1.TolerationOpEqual))
		})
	})

	Context("when parsing invalid toleration strings", func() {
		It("should return error for missing parts", func() {
			_, err := parseToleration("key:value")
			Expect(err).To(HaveOccurred())
		})

		It("should return error for invalid operator", func() {
			_, err := parseToleration("key=value:NoSchedule:Invalid")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("NodeSelector", func() {
	It("should handle single node selector", func() {
		input := map[string]string{"disktype": "ssd"}
		Expect(input).To(Equal(map[string]string{"disktype": "ssd"}))
	})

	It("should handle multiple node selectors", func() {
		input := map[string]string{
			"disktype":    "ssd",
			"environment": "production",
		}
		Expect(input).To(HaveKeyWithValue("disktype", "ssd"))
		Expect(input).To(HaveKeyWithValue("environment", "production"))
	})

	It("should handle node selector with empty value", func() {
		input := map[string]string{"node-role.kubernetes.io/control-plane": ""}
		Expect(input).To(HaveKeyWithValue("node-role.kubernetes.io/control-plane", ""))
	})

	It("should handle complex node selector keys", func() {
		input := map[string]string{
			"topology.kubernetes.io/zone":      "us-east-1a",
			"node.kubernetes.io/instance-type": "m5.large",
		}
		Expect(input).To(HaveKeyWithValue("topology.kubernetes.io/zone", "us-east-1a"))
		Expect(input).To(HaveKeyWithValue("node.kubernetes.io/instance-type", "m5.large"))
	})

	It("should handle empty node selector", func() {
		input := map[string]string{}
		Expect(input).To(BeEmpty())
	})
})
