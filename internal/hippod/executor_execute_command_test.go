package hippod

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hiperrors"
)

// installTerminalWatchReactor wires a watcher that, on the first Watch call,
// delivers a Modify event carrying the supplied terminal pod. waitForPodCompletion
// blocks on the Watch ResultChan, so this is the minimum plumbing to drive
// ExecuteCommand all the way through to a clean return on the fake clientset.
func installTerminalWatchReactor(cs withReactor, terminal *corev1.Pod) {
	fakeWatcher := watch.NewFakeWithChanSize(1, false)
	delivered := false
	cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
		if !delivered {
			delivered = true
			go func() {
				time.Sleep(10 * time.Millisecond)
				fakeWatcher.Modify(terminal)
			}()
		}
		return true, fakeWatcher, nil
	})
}

// withReactor narrows the fake.Clientset surface to just the bits we need so
// the helper composes cleanly with anything that exposes PrependWatchReactor.
type withReactor interface {
	PrependWatchReactor(resource string, reaction clienttesting.WatchReactionFunc)
}

var _ = Describe("ExecuteCommand", func() {
	const ns = "helm-in-pod"

	// ──────────────────────────────────────────────────────────────────────
	// Path 1 — scriptPreCopied=true (fast path: mv staged → wrapped)
	// ──────────────────────────────────────────────────────────────────────

	Describe("scriptPreCopied=true (fast path)", func() {
		It("sends 'mv <staged> <wrapped>' command when CopyAttempts>=1", func() {
			m, cap, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p1", ns)
			opts := cmdoptions.ExecOptions{CopyAttempts: 1}

			// ExecInPod will fail (404), so ExecuteCommand returns early with
			// that error before reaching the streaming loop. We only assert
			// the command shape here.
			err := m.ExecuteCommand(context.Background(), pod, "helm list", opts, true)
			Expect(err).To(HaveOccurred())

			cmds := cap.commands()
			Expect(cmds).NotTo(BeEmpty())
			Expect(cmds[0]).To(ContainSubstring("mv " + hipconsts.StagedScriptPath))
			Expect(cmds[0]).To(ContainSubstring(hipconsts.WrappedScriptPath))
		})

		It("skips the mv exec entirely when CopyAttempts=0 and reaches the streaming loop", func() {
			// CopyAttempts=0 short-circuits the retry helper to nil, so the mv
			// step is treated as a no-op. The loop then exits via the
			// terminal-phase branch and waitForPodCompletion returns nil.
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			m, cs := newManagerWithFakeClientAndPod(succeeded)
			installTerminalWatchReactor(cs, succeeded)

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			Expect(m.ExecuteCommand(context.Background(), succeeded, "helm list", opts, true)).
				To(Succeed())
		})
	})

	// ──────────────────────────────────────────────────────────────────────
	// Path 2 — scriptPreCopied=false (slow path: temp file → CopyFileToPod)
	// ──────────────────────────────────────────────────────────────────────

	Describe("scriptPreCopied=false (slow path)", func() {
		It("writes a temp wrapper script and forwards it via tar to the pod", func() {
			m, cap, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p1", ns)
			opts := cmdoptions.ExecOptions{CopyAttempts: 1}

			err := m.ExecuteCommand(context.Background(), pod, "helm list", opts, false)
			Expect(err).To(HaveOccurred(),
				"CopyFileToPod is expected to fail against the 404 capturing server")

			cmds := cap.commands()
			Expect(cmds).NotTo(BeEmpty())
			// CopyFileToPod sends "mkdir -p <dir> && tar zxf - -C /" where dir
			// is the parent of WrappedScriptPath.
			Expect(cmds[0]).To(ContainSubstring("mkdir -p /tmp"))
			Expect(cmds[0]).To(ContainSubstring("tar zxf -"))
		})

		It("returns the CopyFileToPod error when CopyAttempts>=1 and exec fails", func() {
			m, _, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p1", ns)
			opts := cmdoptions.ExecOptions{CopyAttempts: 1}

			err := m.ExecuteCommand(context.Background(), pod, "helm list", opts, false)
			Expect(err).To(HaveOccurred())
		})

		It("skips the tar exec entirely when CopyAttempts=0 and reaches the streaming loop", func() {
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			m, cs := newManagerWithFakeClientAndPod(succeeded)
			installTerminalWatchReactor(cs, succeeded)

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			Expect(m.ExecuteCommand(context.Background(), succeeded, "helm list", opts, false)).
				To(Succeed())
		})
	})

	// ──────────────────────────────────────────────────────────────────────
	// Timeout goroutine — sends `kill -term 1` on DeadlineExceeded
	// ──────────────────────────────────────────────────────────────────────

	Describe("timeout goroutine", func() {
		It("sends 'kill -term 1' into the pod when ctx hits its deadline", func() {
			m, cap, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p1", ns)
			// CopyAttempts=0 skips the staged-script move; ExecuteCommand then
			// enters the streaming loop where every API call against the 404
			// server is a no-op, and the deadline-watching goroutine fires.
			opts := cmdoptions.ExecOptions{CopyAttempts: 0}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()

			_ = m.ExecuteCommand(ctx, pod, "helm list", opts, true)

			// The detached goroutine keeps trying kill -term 1 up to 20×50ms,
			// which is well within a 2s window.
			Eventually(func() []string { return cap.commands() }, "2s", "20ms").
				Should(ContainElement(ContainSubstring("kill -term 1")))
		})

		It("does NOT send 'kill -term 1' when ctx is canceled without a deadline", func() {
			m, cap, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p1", ns)
			opts := cmdoptions.ExecOptions{CopyAttempts: 0}

			ctx, cancel := context.WithCancel(context.Background())
			// Cancel before invocation so the goroutine wakes immediately on
			// ctx.Err() == context.Canceled and exits without issuing kill.
			cancel()
			_ = m.ExecuteCommand(ctx, pod, "helm list", opts, true)

			// Give the detached goroutine a moment to (not) run. 200ms is well
			// above the goroutine's wake latency on any sane scheduler.
			time.Sleep(200 * time.Millisecond)
			for _, c := range cap.commands() {
				Expect(c).NotTo(ContainSubstring("kill -term 1"),
					"the goroutine must short-circuit on plain context.Canceled")
			}
		})
	})

	// ──────────────────────────────────────────────────────────────────────
	// Copy-from mode wiring
	// ──────────────────────────────────────────────────────────────────────

	Describe("copy-from mode wiring", func() {
		It("returns ExitCodeError when CopyFrom is set and the pod ends Failed", func() {
			// We cannot make fake GetLogs emit the exit-code marker (it hard-
			// codes "fake logs"), so the marker-writer path stays unfound.
			// In that case ExecuteCommand falls through to waitForPodCompletion,
			// which surfaces the pod's terminated exit code.
			failed := makePod("p1", ns, corev1.PodFailed, 7)
			m, cs := newManagerWithFakeClientAndPod(failed)
			installTerminalWatchReactor(cs, failed)

			opts := cmdoptions.ExecOptions{
				CopyAttempts: 0,
				CopyFrom:     []string{"/workspace/artifact.tgz"},
			}

			err := m.ExecuteCommand(context.Background(), failed, "helm pull", opts, true)

			var exitErr *hiperrors.ExitCodeError
			Expect(errors.As(err, &exitErr)).To(BeTrue(),
				"copy-from mode must still bubble up the pod-level exit code on Failure")
			Expect(exitErr.Code).To(Equal(int32(7)))
		})

		It("returns nil when CopyFrom is set and the pod ends Succeeded", func() {
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			m, cs := newManagerWithFakeClientAndPod(succeeded)
			installTerminalWatchReactor(cs, succeeded)

			opts := cmdoptions.ExecOptions{
				CopyAttempts: 0,
				CopyFrom:     []string{"/workspace/artifact.tgz"},
			}
			Expect(m.ExecuteCommand(context.Background(), succeeded, "helm pull", opts, true)).
				To(Succeed())
		})

		It("masks --set values in the log line when SuppressSecrets is on (smoke check)", func() {
			// Drives both SuppressSecrets and the normal mode return path in
			// one go. The log message itself is not asserted (zerolog state
			// isn't part of the contract), but the function must not panic
			// on the masked-display branch.
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			m, cs := newManagerWithFakeClientAndPod(succeeded)
			installTerminalWatchReactor(cs, succeeded)

			opts := cmdoptions.ExecOptions{
				CopyAttempts:    0,
				SuppressSecrets: true,
			}
			Expect(m.ExecuteCommand(context.Background(), succeeded,
				`helm install --set password=topsecret`, opts, true)).To(Succeed())
		})
	})

	// ──────────────────────────────────────────────────────────────────────
	// wg.Go loop — phase handling
	// ──────────────────────────────────────────────────────────────────────

	Describe("wg.Go phase loop", func() {
		It("returns nil when pod is already in PodSucceeded phase (terminal branch)", func() {
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			m, cs := newManagerWithFakeClientAndPod(succeeded)
			installTerminalWatchReactor(cs, succeeded)

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			Expect(m.ExecuteCommand(context.Background(), succeeded, "helm list", opts, true)).
				To(Succeed())
		})

		It("returns ExitCodeError when pod is in PodFailed phase with a non-zero exit", func() {
			failed := makePod("p1", ns, corev1.PodFailed, 137)
			m, cs := newManagerWithFakeClientAndPod(failed)
			installTerminalWatchReactor(cs, failed)

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			err := m.ExecuteCommand(context.Background(), failed, "helm list", opts, true)

			var exitErr *hiperrors.ExitCodeError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.Code).To(Equal(int32(137)))
		})

		It("exits the wg loop when GetPodPhase returns NotFound", func() {
			// Empty fake clientset → Get returns IsNotFound, IgnoreNotFound
			// folds it to nil, and the wg.Go loop returns immediately.
			m, cs := newManagerWithFakeClient()
			// Watch reactor delivers a terminal Succeeded event so the
			// subsequent waitForPodCompletion call returns cleanly even
			// though no pod object exists in the tracker.
			installTerminalWatchReactor(cs, makePod("p1", ns, corev1.PodSucceeded, 0))

			pod := testPod("p1", ns) // not in cs.Tracker → Get → 404
			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			Expect(m.ExecuteCommand(context.Background(), pod, "helm list", opts, true)).
				To(Succeed())
		})

		It("returns ctx.Err() when the context is canceled before any terminal event", func() {
			running := makePod("p1", ns, corev1.PodRunning, 0)
			m, _ := newManagerWithFakeClientAndPod(running)

			// No watch reactor is installed → fake client returns its
			// built-in empty watch that simply blocks. We let the context's
			// cancellation drive the exit instead.
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			err := m.ExecuteCommand(ctx, running, "helm list", opts, true)
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	// ──────────────────────────────────────────────────────────────────────
	// Integration — both fast and slow paths exit cleanly when streaming
	// hits a healthy terminal pod
	// ──────────────────────────────────────────────────────────────────────

	Describe("end-to-end (CopyAttempts=0)", func() {
		It("fast path returns nil when pod is Succeeded and Watch delivers it after a brief delay", func() {
			seed := makePod("p1", ns, corev1.PodRunning, 0)
			m, cs := newManagerWithFakeClientAndPod(seed)

			fakeWatcher := watch.NewFakeWithChanSize(1, false)
			cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
				return true, fakeWatcher, nil
			})
			go func() {
				time.Sleep(25 * time.Millisecond)
				_, _ = cs.CoreV1().Pods(ns).Update(
					context.Background(),
					makePod("p1", ns, corev1.PodSucceeded, 0),
					metav1.UpdateOptions{},
				)
				fakeWatcher.Modify(makePod("p1", ns, corev1.PodSucceeded, 0))
			}()

			opts := cmdoptions.ExecOptions{CopyAttempts: 0}
			Expect(m.ExecuteCommand(context.Background(), seed, "helm list", opts, true)).
				To(Succeed())
		})
	})
})
