package hippod

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"

	"github.com/noksa/helm-in-pod/internal/hiperrors"
)

func makePod(name, namespace string, phase corev1.PodPhase, exitCode int32) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			ResourceVersion: "1",
		},
		Status: corev1.PodStatus{Phase: phase},
	}
	if phase == corev1.PodFailed {
		p.Status.ContainerStatuses = []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: exitCode},
				},
			},
		}
	}
	return p
}

var _ = Describe("waitForPodCompletion", func() {
	const ns = "helm-in-pod"

	It("returns nil when watch delivers a PodSucceeded event", func() {
		m, cs := newManagerWithFakeClient()
		fakeWatcher := watch.NewFakeWithChanSize(1, false)
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, fakeWatcher, nil
		})

		seed := makePod("p1", ns, corev1.PodPending, 0)
		go func() {
			time.Sleep(20 * time.Millisecond)
			succeeded := makePod("p1", ns, corev1.PodSucceeded, 0)
			fakeWatcher.Modify(succeeded)
		}()

		err := m.waitForPodCompletion(context.Background(), seed)
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an ExitCodeError when watch delivers a PodFailed event with exit code", func() {
		m, cs := newManagerWithFakeClient()
		fakeWatcher := watch.NewFakeWithChanSize(1, false)
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, fakeWatcher, nil
		})

		seed := makePod("p1", ns, corev1.PodPending, 0)
		go func() {
			time.Sleep(20 * time.Millisecond)
			failed := makePod("p1", ns, corev1.PodFailed, 137)
			fakeWatcher.Modify(failed)
		}()

		err := m.waitForPodCompletion(context.Background(), seed)
		Expect(err).To(HaveOccurred())
		var exitErr *hiperrors.ExitCodeError
		Expect(errors.As(err, &exitErr)).To(BeTrue())
		Expect(exitErr.Code).To(Equal(int32(137)))
	})

	It("returns ctx.Err() when context is canceled before any event arrives", func() {
		m, cs := newManagerWithFakeClient()
		fakeWatcher := watch.NewFakeWithChanSize(0, false)
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, fakeWatcher, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		seed := makePod("p1", ns, corev1.PodPending, 0)
		err := m.waitForPodCompletion(ctx, seed)
		Expect(err).To(MatchError(context.Canceled))
	})

	It("ignores non-terminal phases (Running, Pending) and waits for terminal", func() {
		m, cs := newManagerWithFakeClient()
		fakeWatcher := watch.NewFakeWithChanSize(3, false)
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, fakeWatcher, nil
		})

		seed := makePod("p1", ns, corev1.PodPending, 0)
		go func() {
			fakeWatcher.Modify(makePod("p1", ns, corev1.PodPending, 0))
			fakeWatcher.Modify(makePod("p1", ns, corev1.PodRunning, 0))
			time.Sleep(10 * time.Millisecond)
			fakeWatcher.Modify(makePod("p1", ns, corev1.PodSucceeded, 0))
		}()

		Expect(m.waitForPodCompletion(context.Background(), seed)).To(Succeed())
	})

	It("falls back to polling when the watch call returns an error", func() {
		m, cs := newManagerWithFakeClient()
		seed := makePod("p1", ns, corev1.PodPending, 0)
		_, err := cs.CoreV1().Pods(ns).Create(context.Background(), seed, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, nil, errors.New("watch unavailable: forbidden")
		})

		go func() {
			time.Sleep(20 * time.Millisecond)
			updated := makePod("p1", ns, corev1.PodSucceeded, 0)
			_, _ = cs.CoreV1().Pods(ns).Update(context.Background(), updated, metav1.UpdateOptions{})
		}()

		Expect(m.waitForPodCompletion(context.Background(), seed)).To(Succeed())
	})

	It("returns terminal phase directly from Get when watch channel closes and pod is already Succeeded", func() {
		m, cs := newManagerWithFakeClient()

		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodPending, 0), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		firstWatcher := watch.NewFakeWithChanSize(1, false)
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			return true, firstWatcher, nil
		})

		go func() {
			time.Sleep(20 * time.Millisecond)
			_, _ = cs.CoreV1().Pods(ns).Update(context.Background(),
				makePod("p1", ns, corev1.PodSucceeded, 0), metav1.UpdateOptions{})
			firstWatcher.Stop()
		}()

		seed := makePod("p1", ns, corev1.PodPending, 0)
		Expect(m.waitForPodCompletion(context.Background(), seed)).To(Succeed())
	})

	It("re-opens the watch when channel closes mid-wait and catches terminal event on the new watcher", func() {
		m, cs := newManagerWithFakeClient()

		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodPending, 0), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		firstWatcher := watch.NewFakeWithChanSize(1, false)
		secondWatcher := watch.NewFakeWithChanSize(1, false)
		watchers := []*watch.FakeWatcher{firstWatcher, secondWatcher}
		call := 0
		cs.PrependWatchReactor("pods", func(_ clienttesting.Action) (bool, watch.Interface, error) {
			w := watchers[call]
			call++
			return true, w, nil
		})

		go func() {
			time.Sleep(20 * time.Millisecond)
			firstWatcher.Stop()
			time.Sleep(20 * time.Millisecond)
			secondWatcher.Modify(makePod("p1", ns, corev1.PodSucceeded, 0))
		}()

		seed := makePod("p1", ns, corev1.PodPending, 0)
		Expect(m.waitForPodCompletion(context.Background(), seed)).To(Succeed())
		Expect(call).To(Equal(2), "watch should have been re-opened exactly once")
	})
})

var _ = Describe("pollUntilPodCompletes", func() {
	const ns = "helm-in-pod"

	It("returns nil when the pod reaches PodSucceeded", func() {
		m, cs := newManagerWithFakeClient()
		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodSucceeded, 0), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		seed := makePod("p1", ns, corev1.PodPending, 0)
		Expect(m.pollUntilPodCompletes(context.Background(), seed)).To(Succeed())
	})

	It("returns ExitCodeError when the pod is PodFailed", func() {
		m, cs := newManagerWithFakeClient()
		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodFailed, 42), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		seed := makePod("p1", ns, corev1.PodPending, 0)
		got := m.pollUntilPodCompletes(context.Background(), seed)
		Expect(got).To(HaveOccurred())
		var exitErr *hiperrors.ExitCodeError
		Expect(errors.As(got, &exitErr)).To(BeTrue())
		Expect(exitErr.Code).To(Equal(int32(42)))
	})

	It("returns ctx.Err() when context is canceled before pod becomes terminal", func() {
		m, cs := newManagerWithFakeClient()
		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodPending, 0), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(30 * time.Millisecond)
			cancel()
		}()

		seed := makePod("p1", ns, corev1.PodPending, 0)
		err = m.pollUntilPodCompletes(ctx, seed)
		Expect(err).To(MatchError(context.Canceled))
	})

	It("continues polling when Get returns a transient error then succeeds", func() {
		m, cs := newManagerWithFakeClient()
		_, err := cs.CoreV1().Pods(ns).Create(context.Background(),
			makePod("p1", ns, corev1.PodPending, 0), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		var attempts int32
		cs.PrependReactor("get", "pods", func(_ clienttesting.Action) (bool, runtime.Object, error) {
			attempts++
			if attempts < 3 {
				return true, nil, errors.New("transient API error")
			}
			return false, nil, nil
		})

		go func() {
			time.Sleep(50 * time.Millisecond)
			_, _ = cs.CoreV1().Pods(ns).Update(context.Background(),
				makePod("p1", ns, corev1.PodSucceeded, 0), metav1.UpdateOptions{})
		}()

		seed := makePod("p1", ns, corev1.PodPending, 0)
		Expect(m.pollUntilPodCompletes(context.Background(), seed)).To(Succeed())
		Expect(attempts).To(BeNumerically(">=", 3))
	})
})
