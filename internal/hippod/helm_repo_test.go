package hippod

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientruntime "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// execCapture records all URLs received by the fake exec HTTP server.
type execCapture struct {
	mu   sync.Mutex
	urls []*url.URL
}

func (c *execCapture) add(u *url.URL) {
	c.mu.Lock()
	c.urls = append(c.urls, u)
	c.mu.Unlock()
}

func (c *execCapture) commands() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var cmds []string
	for _, u := range c.urls {
		// The exec request encodes the command as "command" query params.
		params := u.Query()["command"]
		cmds = append(cmds, strings.Join(params, " "))
	}
	return cmds
}

// newCapturingManager creates a Manager whose ExecInPod calls are forwarded to
// an httptest.Server that records the request URLs. The server returns 404 which
// causes ExecInPod to fail — callers that only care about command construction
// should use UpdateRepoAttempts=1 and check errors separately.
func newCapturingManager() (*Manager, *execCapture, *httptest.Server, *fake.Clientset) {
	cap := &execCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.add(r.URL)
		w.WriteHeader(http.StatusNotFound)
	}))
	cfg := &rest.Config{Host: server.URL}
	cs, err := kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	dyn := dynamicfake.NewSimpleDynamicClient(clientruntime.Scheme)
	client := operatorkclient.NewClientFromClientSet(cs, dyn, cfg)
	m := &Manager{
		ctx:          context.Background(),
		myHostname:   "test-host",
		invocationID: "test-invocation",
		kclient:      client,
	}
	// Install a fake k8s clientset alongside for operations that need Get/Update
	// (like AnnotatePod). We use the same fake clientset but note: ExecInPod uses
	// the real http client pointing to the test server, not the fake clientset's
	// in-memory store. Annotation tests use a separate manager with pure fake client.
	return m, cap, server, nil
}

// newManagerWithFakeClientAndPod creates a Manager with a pre-seeded fake clientset
// containing a given pod. UpdateRepoAttempts=0 is used to skip exec calls entirely.
func newManagerWithFakeClientAndPod(pod *corev1.Pod) (*Manager, *fake.Clientset) {
	cs := fake.NewClientset(pod)
	installGenerateNameReactor(cs)
	installDeleteCollectionReactor(cs)
	dyn := dynamicfake.NewSimpleDynamicClient(clientruntime.Scheme, pod)
	client := operatorkclient.NewClientFromClientSet(cs, dyn, nil)
	return &Manager{
		ctx:          context.Background(),
		myHostname:   "test-host",
		invocationID: "test-invocation",
		kclient:      client,
	}, cs
}

func testPod(name, ns string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       ns,
			ResourceVersion: "1",
		},
	}
}

var _ = Describe("updateHelmRepositories", func() {
	ns := hipconsts.Namespace

	Describe("command construction", func() {
		// Use a capturing httptest server — exec will fail (404) so we only
		// check what URLs / query params were sent. UpdateRepoAttempts=1 to
		// actually attempt exec once.
		var (
			m      *Manager
			cap    *execCapture
			server *httptest.Server
		)

		BeforeEach(func() {
			m, cap, server, _ = newCapturingManager()
		})

		AfterEach(func() {
			server.Close()
		})

		It("sends 'helm repo update' (no --fail-on-repo-update-fail) when UpdateRepo is empty", func() {
			pod := testPod("p", ns)
			opts := cmdoptions.ExecOptions{UpdateRepo: nil, UpdateRepoAttempts: 1}
			_ = m.updateHelmRepositories(pod, opts)
			cmds := cap.commands()
			Expect(cmds).To(HaveLen(1))
			Expect(cmds[0]).To(ContainSubstring("helm repo update"))
			Expect(cmds[0]).NotTo(ContainSubstring("--fail-on-repo-update-fail"))
		})

		It("sends 'helm repo update <repo>' per entry when UpdateRepo is non-empty", func() {
			pod := testPod("p", ns)
			opts := cmdoptions.ExecOptions{UpdateRepo: []string{"stable", "bitnami"}, UpdateRepoAttempts: 1}
			_ = m.updateHelmRepositories(pod, opts)
			cmds := cap.commands()
			Expect(cmds).To(HaveLen(2))
			Expect(cmds[0]).To(ContainSubstring("helm repo update stable"))
			Expect(cmds[0]).NotTo(ContainSubstring("--fail-on-repo-update-fail"))
			Expect(cmds[1]).To(ContainSubstring("helm repo update bitnami"))
			Expect(cmds[1]).NotTo(ContainSubstring("--fail-on-repo-update-fail"))
		})
	})

	Describe("error accumulation with errors.Join", func() {
		// Errors from multiple repos must all be returned — not just the first.
		It("returns nil when UpdateRepoAttempts is 0 (no exec performed)", func() {
			m, _ := newManagerWithFakeClientAndPod(testPod("p", ns))
			opts := cmdoptions.ExecOptions{
				UpdateRepo:         []string{"repo1", "repo2"},
				UpdateRepoAttempts: 0,
			}
			err := m.updateHelmRepositories(testPod("p", ns), opts)
			Expect(err).NotTo(HaveOccurred())
		})

		It("accumulates errors from all repos and joins them", func() {
			// Use a capturing server that always returns 404 → exec errors.
			m, _, server, _ := newCapturingManager()
			defer server.Close()

			pod := testPod("p", ns)
			opts := cmdoptions.ExecOptions{
				UpdateRepo:         []string{"repo1", "repo2", "repo3"},
				UpdateRepoAttempts: 1,
			}
			err := m.updateHelmRepositories(pod, opts)
			Expect(err).To(HaveOccurred())

			// errors.Join produces a single error whose message is all sub-errors
			// joined with newlines.
			combined := err.Error()
			for _, repo := range opts.UpdateRepo {
				Expect(combined).To(ContainSubstring(fmt.Sprintf("helm repo update %s", repo)))
			}
		})

		It("returns nil when all repos succeed (UpdateRepoAttempts=0)", func() {
			m, _ := newManagerWithFakeClientAndPod(testPod("p", ns))
			opts := cmdoptions.ExecOptions{
				UpdateRepo:         []string{"stable", "bitnami", "jetstack"},
				UpdateRepoAttempts: 0,
			}
			Expect(m.updateHelmRepositories(testPod("p", ns), opts)).NotTo(HaveOccurred())
		})

		It("returns nil when UpdateRepo is empty and UpdateRepoAttempts=0", func() {
			m, _ := newManagerWithFakeClientAndPod(testPod("p", ns))
			opts := cmdoptions.ExecOptions{UpdateRepo: nil, UpdateRepoAttempts: 0}
			Expect(m.updateHelmRepositories(testPod("p", ns), opts)).NotTo(HaveOccurred())
		})

		// Verify errors.Join unwrapping yields individual errors for each repo.
		It("joined error contains one sub-error per failed repo", func() {
			m, _, server, _ := newCapturingManager()
			defer server.Close()

			opts := cmdoptions.ExecOptions{
				UpdateRepo:         []string{"alpha", "beta"},
				UpdateRepoAttempts: 1,
			}
			err := m.updateHelmRepositories(testPod("p", ns), opts)
			Expect(err).To(HaveOccurred())

			// errors.Join creates an interface with Unwrap() []error
			type unwrapper interface{ Unwrap() []error }
			uw, ok := err.(unwrapper)
			Expect(ok).To(BeTrue(), "expected errors.Join result")
			Expect(uw.Unwrap()).To(HaveLen(2))
		})
	})
})

var _ = Describe("UpdateHelmRepositories vs SyncHelmRepositories annotation asymmetry", func() {
	ns := hipconsts.Namespace

	// Both tests use UpdateRepoAttempts=0 so no exec is attempted.
	// AnnotatePod uses the k8s fake clientset's Get+Update which works without SPDY.

	It("UpdateHelmRepositories annotates the pod with AnnotationLastRepoUpdateTime", func() {
		pod := testPod("annot-pod", ns)
		m, cs := newManagerWithFakeClientAndPod(pod)

		opts := cmdoptions.ExecOptions{UpdateRepo: nil, UpdateRepoAttempts: 0}
		Expect(m.UpdateHelmRepositories(pod, opts)).NotTo(HaveOccurred())

		updated, err := cs.CoreV1().Pods(ns).Get(context.Background(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Annotations).To(HaveKey(hipconsts.AnnotationLastRepoUpdateTime))
		Expect(updated.Annotations[hipconsts.AnnotationLastRepoUpdateTime]).NotTo(BeEmpty())
	})

	It("SyncHelmRepositories does NOT annotate the pod", func() {
		pod := testPod("sync-pod", ns)
		m, cs := newManagerWithFakeClientAndPod(pod)

		// repoPreCopied=true skips the file-copy path and calls updateHelmRepositories directly.
		opts := cmdoptions.ExecOptions{UpdateRepo: nil, UpdateRepoAttempts: 0}
		Expect(m.SyncHelmRepositories(pod, opts, "/home/user", true)).NotTo(HaveOccurred())

		// Pod must NOT have the annotation.
		unchanged, err := cs.CoreV1().Pods(ns).Get(context.Background(), pod.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(unchanged.Annotations).NotTo(HaveKey(hipconsts.AnnotationLastRepoUpdateTime))
	})
})
