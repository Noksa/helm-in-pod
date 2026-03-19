package hippod

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("parseExitCodeFromError", func() {
	It("should return -1 for nil error", func() {
		Expect(parseExitCodeFromError(nil)).To(Equal(-1))
	})

	It("should parse 'command terminated with exit code 2'", func() {
		err := fmt.Errorf("command terminated with exit code 2")
		Expect(parseExitCodeFromError(err)).To(Equal(2))
	})

	It("should parse wrapped error from operatorkclient", func() {
		err := fmt.Errorf("'sh /tmp/wrapped-script.sh' command failed: command terminated with exit code 137")
		Expect(parseExitCodeFromError(err)).To(Equal(137))
	})

	It("should return -1 for unrelated error", func() {
		err := fmt.Errorf("connection refused")
		Expect(parseExitCodeFromError(err)).To(Equal(-1))
	})

	It("should parse exit code 0", func() {
		err := fmt.Errorf("command terminated with exit code 0")
		Expect(parseExitCodeFromError(err)).To(Equal(0))
	})

	It("should parse exit code 1", func() {
		err := fmt.Errorf("command terminated with exit code 1")
		Expect(parseExitCodeFromError(err)).To(Equal(1))
	})

	It("should parse exit code 255", func() {
		err := fmt.Errorf("command terminated with exit code 255")
		Expect(parseExitCodeFromError(err)).To(Equal(255))
	})

	It("should handle trailing whitespace after code", func() {
		err := fmt.Errorf("command terminated with exit code 42  ")
		Expect(parseExitCodeFromError(err)).To(Equal(42))
	})

	It("should return -1 when 'exit code' prefix exists but no digits follow", func() {
		err := fmt.Errorf("something exit code abc")
		Expect(parseExitCodeFromError(err)).To(Equal(-1))
	})
})

var _ = Describe("exitCodeFromContainerStatuses", func() {
	It("should return the exit code from a terminated container", func() {
		statuses := []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 2},
				},
			},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(2)))
	})

	It("should return exit code 0 for a successfully terminated container", func() {
		statuses := []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(0)))
	})

	It("should return -1 when container is still running", func() {
		statuses := []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(-1)))
	})

	It("should return -1 for empty statuses", func() {
		Expect(exitCodeFromContainerStatuses(nil)).To(Equal(int32(-1)))
		Expect(exitCodeFromContainerStatuses([]corev1.ContainerStatus{})).To(Equal(int32(-1)))
	})

	It("should return the first terminated container's exit code", func() {
		statuses := []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			},
			{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 137},
				},
			},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(137)))
	})

	It("should return -1 when container is waiting", func() {
		statuses := []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				},
			},
		}
		Expect(exitCodeFromContainerStatuses(statuses)).To(Equal(int32(-1)))
	})
})
