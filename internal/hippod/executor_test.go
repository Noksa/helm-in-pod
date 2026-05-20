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

	It("forwards subsequent writes verbatim after marker is found", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:5###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())

		_, err = w.Write([]byte("trailing output\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(inner.String()).To(Equal("trailing output\n"))
	})

	It("only invokes the cancel function once even on subsequent writes", func() {
		_, _ = w.Write([]byte("###HIP_EXIT_CODE:1###\n"))
		_, _ = w.Write([]byte("more output\n"))
		_, _ = w.Write([]byte("###HIP_EXIT_CODE:99###\n"))
		Expect(cancelCalls).To(Equal(1))
		Expect(w.ExitCode()).To(Equal(1), "first marker wins")
	})

	It("locks in current behavior: marker split across two writes silently loses the exit code", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse())
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty(),
			"first chunk containing only the prefix is currently consumed but not registered")

		_, err = w.Write([]byte("42###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"second chunk has no prefix so the exit code is lost — KNOWN LIMITATION")
		Expect(inner.String()).To(Equal("42###\n"),
			"second chunk leaks the suffix as normal output")
	})

	It("locks in current behavior: non-numeric marker value silently consumes the line", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:abc###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse())
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty(),
			"line is dropped: not forwarded and not registered — KNOWN LIMITATION")
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

	It("locks in current behavior: stdout preceding the marker in the same Write is silently dropped", func() {
		// In production, kubelet log batching can deliver stdout and the
		// marker in a single chunk. The writer recognizes the marker and
		// consumes the entire buffer — preceding user output is lost.
		_, err := w.Write([]byte("important user output\n###HIP_EXIT_CODE:9###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(9))
		Expect(inner.String()).To(BeEmpty(),
			"preceding stdout is dropped — KNOWN LIMITATION")
	})

	It("locks in current behavior: stdout following the marker in the same Write loses both", func() {
		// Symmetric to the preceding-stdout case: when the marker has
		// trailing output in the same Write, CutSuffix fails so the line
		// is consumed but never registered.
		_, err := w.Write([]byte("###HIP_EXIT_CODE:5###\nstdout after marker\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"marker is consumed but not registered when trailing output is present — KNOWN LIMITATION")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty(),
			"trailing stdout is dropped together with the marker")
	})

	It("locks in current behavior: two markers in a single Write register neither", func() {
		// strings.Cut splits on the FIRST prefix, leaving the second marker
		// embedded in the value — strconv.Atoi then fails on the embedded
		// '#' characters.
		_, err := w.Write([]byte("###HIP_EXIT_CODE:1###\n###HIP_EXIT_CODE:2###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"two markers in one Write are dropped together — KNOWN LIMITATION")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty())
	})

	It("locks in current behavior: empty marker value is silently dropped", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"empty value fails Atoi → marker not registered — KNOWN LIMITATION")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty(),
			"line is still consumed (not forwarded)")
	})

	It("locks in current behavior: value that overflows int is silently dropped", func() {
		_, err := w.Write([]byte("###HIP_EXIT_CODE:99999999999999999999999###\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeFalse(),
			"Atoi returns range error → marker not registered — KNOWN LIMITATION")
		Expect(cancelCalls).To(Equal(0))
		Expect(inner.String()).To(BeEmpty())
	})

	It("locks in current behavior: marker without the trailing ### suffix is still accepted", func() {
		// The current code falls back to a bare prefix match when both
		// `suffix+\n` and `suffix` CutSuffix calls return ok=false. The
		// result is that a malformed line missing the suffix is accepted
		// as long as the value parses as an int. Behavior is surprising
		// but matches what the embedded script emits today.
		_, err := w.Write([]byte("###HIP_EXIT_CODE:7\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(w.Found()).To(BeTrue())
		Expect(w.ExitCode()).To(Equal(7))
		Expect(cancelCalls).To(Equal(1))
	})
})
