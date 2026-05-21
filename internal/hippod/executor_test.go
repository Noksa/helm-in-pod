package hippod

import (
	"bytes"
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

var _ = Describe("exitCodeMarkerWriter", func() {
	var (
		inner       *bytes.Buffer
		cancelCalls int
		cancel      func()
		w           *exitCodeMarkerWriter
	)

	BeforeEach(func() {
		inner = &bytes.Buffer{}
		cancelCalls = 0
		cancel = func() { cancelCalls++ }
		w = newExitCodeMarkerWriter(inner, cancel)
	})

	It("starts unfound with exit code zero", func() {
		Expect(w.Found()).To(BeFalse())
		Expect(w.ExitCode()).To(Equal(0))
	})

	It("forwards normal output to the inner writer", func() {
		n, err := w.Write([]byte("hello\nworld\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(12))
		Expect(inner.String()).To(Equal("hello\nworld\n"))
		Expect(w.Found()).To(BeFalse())
		Expect(cancelCalls).To(Equal(0))
	})

	It("captures the exit code from a marker line with trailing newline", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:42###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(42))
		Expect(cancelCalls).To(Equal(1))
		Expect(inner.String()).To(BeEmpty(), "marker must not be forwarded to inner")
	})

	It("captures the exit code from a marker line without trailing newline", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:7###"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(7))
		Expect(cancelCalls).To(Equal(1))
	})

	It("captures exit code 0", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:0###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(0))
		Expect(cancelCalls).To(Equal(1))
	})

	It("captures non-zero exit code 137 (SIGKILL convention)", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:137###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.ExitCode()).To(Equal(137))
	})

	It("discards subsequent writes after marker is found", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:5###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())

		_, err = w.Write([]byte("trailing output\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(inner.String()).To(BeEmpty(),
			"post-marker data is discarded to prevent garbage on stdout")
	})

	It("only invokes the cancel function once even on subsequent writes", func() {
		_, _ = w.Write([]byte("###HIP_EXIT_CODE:1###\n"))
		_, _ = w.Write([]byte("more output\n"))
		_, _ = w.Write([]byte("###HIP_EXIT_CODE:99###\n"))
		Expect(cancelCalls).To(Equal(1))
		Expect(w.ExitCode()).To(Equal(1), "first marker wins")
	})

	It("handles marker split across two writes via line buffering", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(), "no newline yet, line still buffered")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty())

		_, err = w.Write([]byte("42###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue(), "complete line now available, marker detected")
		Expect(w.ExitCode()).To(Equal(42))
		Expect(cancelCalls).To(Equal(1))
	})

	It("non-numeric marker value is not registered and line is forwarded", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:abc###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse())
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(Equal("###HIP_EXIT_CODE:abc###\n"),
			"invalid marker line is forwarded as normal output")
	})

	It("trims whitespace around the exit code value", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:  3  ###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(3))
	})

	It("captures negative exit codes (lock-in: no validation that codes are non-negative)", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:-1###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(-1))
		Expect(cancelCalls).To(Equal(1))
	})

	It("forwards stdout preceding the marker in the same Write", func() {
		_, err := w.Write([]byte("important user output\n###HIP_EXIT_CODE:9###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(9))
		Expect(inner.String()).To(Equal("important user output\n"),
			"preceding stdout is forwarded correctly")
	})

	It("detects marker and discards stdout following it in the same Write", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:5###\nstdout after marker\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(5))
		Expect(cancelCalls).To(Equal(1))
		Expect(inner.String()).To(BeEmpty(),
			"trailing stdout after marker is discarded")
	})

	It("registers the first marker when two markers appear in a single Write", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:1###\n###HIP_EXIT_CODE:2###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(1), "first marker wins")
		Expect(cancelCalls).To(Equal(1))
		Expect(inner.String()).To(BeEmpty(),
			"second marker line is discarded after first is found")
	})

	It("empty marker value is not registered and line is forwarded", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"empty value fails Atoi → marker not registered")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(Equal("###HIP_EXIT_CODE:###\n"),
			"invalid marker line is forwarded as normal output")
	})

	It("overflow int value is not registered and line is forwarded", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:99999999999999999999999###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"Atoi returns range error → marker not registered")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(Equal("###HIP_EXIT_CODE:99999999999999999999999###\n"),
			"invalid marker line is forwarded as normal output")
	})

	It("marker without the trailing ### suffix is rejected and forwarded", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:7\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"missing suffix means it's not a valid marker")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(Equal("###HIP_EXIT_CODE:7\n"),
			"invalid marker line is forwarded as normal output")
	})
})
