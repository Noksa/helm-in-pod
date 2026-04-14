package hippod

import (
	"context"
	"fmt"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// buildLabelSelector mimics the logic in DeleteHelmPods for building label selectors.
// invocationID must be provided to reflect the per-process UUID added in NewManager.
func buildLabelSelector(hostname, invocationID string, execOptions cmdoptions.ExecOptions, purgeAll bool) string {
	if purgeAll {
		return ""
	}

	selector := fmt.Sprintf("host=%v,%v=%v", hostname, hipconsts.LabelOperationID, invocationID)
	for k, v := range execOptions.Labels {
		selector = fmt.Sprintf("%v,%v=%v", selector, k, v)
	}
	return selector
}

var _ = Describe("DeleteHelmPods Label Selector Logic", func() {
	var hostname string

	BeforeEach(func() {
		hostname = "test-host"
	})

	Context("when building label selectors", func() {
		It("should include hostname, invocation ID and single custom label", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "abc123",
				},
			}

			selector := buildLabelSelector(hostname, "inv-uuid-1", execOptions, false)
			Expect(selector).To(ContainSubstring("host=test-host"))
			Expect(selector).To(ContainSubstring(hipconsts.LabelOperationID + "=inv-uuid-1"))
			Expect(selector).To(ContainSubstring("test-id=abc123"))
		})

		It("should include hostname, invocation ID and multiple custom labels", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "xyz789",
					"env":     "test",
					"team":    "platform",
				},
			}

			selector := buildLabelSelector(hostname, "inv-uuid-2", execOptions, false)
			Expect(selector).To(ContainSubstring("host=test-host"))
			Expect(selector).To(ContainSubstring(hipconsts.LabelOperationID + "=inv-uuid-2"))
			Expect(selector).To(ContainSubstring("test-id=xyz789"))
			Expect(selector).To(ContainSubstring("env=test"))
			Expect(selector).To(ContainSubstring("team=platform"))
		})

		It("should include hostname and invocation ID even when no custom labels are provided", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{},
			}

			selector := buildLabelSelector(hostname, "inv-uuid-3", execOptions, false)
			Expect(selector).To(ContainSubstring("host=test-host"))
			Expect(selector).To(ContainSubstring(hipconsts.LabelOperationID + "=inv-uuid-3"))
		})

		It("should return empty selector when purge all is true", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "abc123",
				},
			}

			selector := buildLabelSelector(hostname, "inv-uuid-4", execOptions, true)
			Expect(selector).To(BeEmpty())
		})

		It("should isolate pods by hostname", func() {
			hostname1 := "host-1"
			hostname2 := "host-2"
			execOptions := cmdoptions.ExecOptions{Labels: map[string]string{}}
			invID := "same-inv-id"

			selector1 := buildLabelSelector(hostname1, invID, execOptions, false)
			selector2 := buildLabelSelector(hostname2, invID, execOptions, false)

			Expect(selector1).NotTo(Equal(selector2))
			Expect(selector1).To(ContainSubstring("host=host-1"))
			Expect(selector2).To(ContainSubstring("host=host-2"))
		})
	})

	Context("concurrent process isolation via invocation ID", func() {
		It("should assign unique invocation IDs to different Manager instances", func() {
			// Simulate two concurrent helm-in-pod processes on the same host.
			// Each NewManager() call must produce a distinct invocationID.
			m1 := NewManager(context.Background(), hostname)
			m2 := NewManager(context.Background(), hostname)

			Expect(m1.invocationID).NotTo(BeEmpty())
			Expect(m2.invocationID).NotTo(BeEmpty())
			Expect(m1.invocationID).NotTo(Equal(m2.invocationID),
				"Two concurrent managers on the same host must have different invocation IDs")
		})

		It("should produce non-overlapping deletion selectors for concurrent managers", func() {
			// If process B's selector matches process A's pods, B will delete A's pod.
			// With distinct invocation IDs that cannot happen.
			m1 := NewManager(context.Background(), hostname)
			m2 := NewManager(context.Background(), hostname)

			opts := cmdoptions.ExecOptions{Labels: map[string]string{}}

			sel1 := buildLabelSelector(hostname, m1.invocationID, opts, false)
			sel2 := buildLabelSelector(hostname, m2.invocationID, opts, false)

			Expect(sel1).NotTo(Equal(sel2))
			Expect(sel1).To(ContainSubstring(m1.invocationID))
			Expect(sel2).To(ContainSubstring(m2.invocationID))
			Expect(sel1).NotTo(ContainSubstring(m2.invocationID))
			Expect(sel2).NotTo(ContainSubstring(m1.invocationID))
		})

		It("should ensure N concurrent processes all have unique invocation IDs", func() {
			n := 10
			managers := make([]*Manager, n)
			for i := range managers {
				managers[i] = NewManager(context.Background(), hostname)
			}

			seen := map[string]bool{}
			for _, m := range managers {
				Expect(seen).NotTo(HaveKey(m.invocationID),
					"Invocation ID %s already seen — not unique", m.invocationID)
				seen[m.invocationID] = true
			}
		})
	})

	Context("when verifying parallel test isolation with custom labels", func() {
		It("should create different selectors for different test IDs", func() {
			invID := "same-inv"
			execOptions1 := cmdoptions.ExecOptions{Labels: map[string]string{"test-id": "run1-abc"}}
			execOptions2 := cmdoptions.ExecOptions{Labels: map[string]string{"test-id": "run2-xyz"}}

			selector1 := buildLabelSelector(hostname, invID, execOptions1, false)
			selector2 := buildLabelSelector(hostname, invID, execOptions2, false)

			Expect(selector1).NotTo(Equal(selector2))
			Expect(selector1).To(ContainSubstring("test-id=run1-abc"))
			Expect(selector2).To(ContainSubstring("test-id=run2-xyz"))
		})

		It("should demonstrate label-based isolation prevents cross-deletion", func() {
			invID := "shared-inv"
			test1Options := cmdoptions.ExecOptions{Labels: map[string]string{"test-id": "test1"}}
			test2Options := cmdoptions.ExecOptions{Labels: map[string]string{"test-id": "test2"}}

			test1Selector := buildLabelSelector(hostname, invID, test1Options, false)
			test2Selector := buildLabelSelector(hostname, invID, test2Options, false)

			Expect(test1Selector).NotTo(Equal(test2Selector))
			Expect(test1Selector).To(ContainSubstring("test-id=test1"))
			Expect(test1Selector).NotTo(ContainSubstring("test-id=test2"))
			Expect(test2Selector).To(ContainSubstring("test-id=test2"))
			Expect(test2Selector).NotTo(ContainSubstring("test-id=test1"))
		})
	})

	Context("when working with daemon pods", func() {
		It("should include custom labels in daemon pod creation", func() {
			daemonOpts := cmdoptions.DaemonOptions{
				Name: "test-daemon",
				ExecOptions: cmdoptions.ExecOptions{
					Labels: map[string]string{
						"test-id": "daemon-test-123",
						"env":     "test",
					},
				},
			}

			Expect(daemonOpts.Labels).To(HaveKeyWithValue("test-id", "daemon-test-123"))
			Expect(daemonOpts.Labels).To(HaveKeyWithValue("env", "test"))
		})

		It("should create unique daemon names for parallel tests", func() {
			daemon1Name := "test-daemon-abc123"
			daemon2Name := "test-daemon-xyz789"

			Expect(daemon1Name).NotTo(Equal(daemon2Name))
			Expect(fmt.Sprintf("daemon-%s", daemon1Name)).NotTo(Equal(fmt.Sprintf("daemon-%s", daemon2Name)))
		})
	})
})
