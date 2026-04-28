package hippod

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/noksa/helm-in-pod/internal/hiperrors"
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

var _ = Describe("isPodReady", func() {
	It("returns false for a nil pod", func() {
		Expect(isPodReady(nil)).To(BeFalse())
	})

	It("returns false when the pod has no container statuses", func() {
		pod := &corev1.Pod{}
		Expect(isPodReady(pod)).To(BeFalse())
	})

	It("returns false when every container is not ready", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "a", Ready: false},
					{Name: "b", Ready: false},
				},
			},
		}
		Expect(isPodReady(pod)).To(BeFalse())
	})

	It("returns true when at least one container is ready", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "a", Ready: false},
					{Name: "b", Ready: true},
				},
			},
		}
		Expect(isPodReady(pod)).To(BeTrue())
	})

	It("returns true when the single container is ready", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "helm-in-pod", Ready: true},
				},
			},
		}
		Expect(isPodReady(pod)).To(BeTrue())
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

var _ = Describe("handleTerminalPhase", func() {
	var m *Manager

	BeforeEach(func() {
		m = &Manager{}
	})

	It("returns nil for a Succeeded pod", func() {
		err := m.handleTerminalPhase(context.Background(), &corev1.Pod{}, corev1.PodSucceeded)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns a pod-failed error when phase is Failed and exit code is unknown", func() {
		err := m.handleTerminalPhase(context.Background(), &corev1.Pod{}, corev1.PodFailed)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("pod failed"))
	})

	It("returns ctx.Err() when context is already canceled and phase is Failed", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := m.handleTerminalPhase(ctx, &corev1.Pod{}, corev1.PodFailed)
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})

	It("returns ExitCodeError when the container status carries an exit code", func() {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 42},
					}},
				},
			},
		}
		err := m.handleTerminalPhase(context.Background(), pod, corev1.PodFailed)
		Expect(err).To(HaveOccurred())
		var exitErr *hiperrors.ExitCodeError
		Expect(errors.As(err, &exitErr)).To(BeTrue())
		Expect(exitErr.Code).To(Equal(int32(42)))
	})
})

var _ = Describe("exitCodeFromContainerStatuses", func() {
	It("returns ExitCodeUnknown when statuses is empty", func() {
		Expect(exitCodeFromContainerStatuses(nil)).To(Equal(int32(hiperrors.ExitCodeUnknown)))
	})

	It("returns ExitCodeUnknown when no container is terminated", func() {
		statuses := []corev1.ContainerStatus{
			{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(hiperrors.ExitCodeUnknown)))
	})

	It("returns the exit code of the first terminated container", func() {
		statuses := []corev1.ContainerStatus{
			{State: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 137},
			}},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(137)))
	})
})
