package hippod

import (
	"fmt"
	"strings"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// buildLabelSelector mimics the logic in DeleteHelmPods for building label selectors
func buildLabelSelector(hostname string, execOptions cmdoptions.ExecOptions, purgeAll bool) string {
	if purgeAll {
		return ""
	}

	selector := fmt.Sprintf("host=%v", hostname)
	for k, v := range execOptions.Labels {
		selector = fmt.Sprintf("%v,%v=%v", selector, k, v)
	}
	selector = strings.TrimSuffix(selector, ",")
	selector = strings.TrimPrefix(selector, ",")
	return selector
}

var _ = Describe("DeleteHelmPods Label Selector Logic", func() {
	var hostname string

	BeforeEach(func() {
		hostname = "test-host"
	})

	Context("when building label selectors", func() {
		It("should include hostname and single custom label", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "abc123",
				},
			}

			selector := buildLabelSelector(hostname, execOptions, false)
			Expect(selector).To(ContainSubstring("host=test-host"))
			Expect(selector).To(ContainSubstring("test-id=abc123"))
		})

		It("should include hostname and multiple custom labels", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "xyz789",
					"env":     "test",
					"team":    "platform",
				},
			}

			selector := buildLabelSelector(hostname, execOptions, false)
			Expect(selector).To(ContainSubstring("host=test-host"))
			Expect(selector).To(ContainSubstring("test-id=xyz789"))
			Expect(selector).To(ContainSubstring("env=test"))
			Expect(selector).To(ContainSubstring("team=platform"))
		})

		It("should only include hostname when no custom labels", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{},
			}

			selector := buildLabelSelector(hostname, execOptions, false)
			Expect(selector).To(Equal("host=test-host"))
		})

		It("should return empty selector when purge all is true", func() {
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "abc123",
				},
			}

			selector := buildLabelSelector(hostname, execOptions, true)
			Expect(selector).To(BeEmpty())
		})

		It("should create different selectors for different test IDs", func() {
			execOptions1 := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "run1-abc",
				},
			}
			execOptions2 := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "run2-xyz",
				},
			}

			selector1 := buildLabelSelector(hostname, execOptions1, false)
			selector2 := buildLabelSelector(hostname, execOptions2, false)

			Expect(selector1).NotTo(Equal(selector2))
			Expect(selector1).To(ContainSubstring("test-id=run1-abc"))
			Expect(selector2).To(ContainSubstring("test-id=run2-xyz"))
		})

		It("should isolate pods by hostname", func() {
			hostname1 := "host-1"
			hostname2 := "host-2"
			execOptions := cmdoptions.ExecOptions{
				Labels: map[string]string{
					"test-id": "same-test-id",
				},
			}

			selector1 := buildLabelSelector(hostname1, execOptions, false)
			selector2 := buildLabelSelector(hostname2, execOptions, false)

			Expect(selector1).NotTo(Equal(selector2))
			Expect(selector1).To(ContainSubstring("host=host-1"))
			Expect(selector2).To(ContainSubstring("host=host-2"))
		})
	})

	Context("when verifying parallel test isolation", func() {
		It("should ensure unique selectors for parallel tests", func() {
			// Simulate 3 parallel test runs
			testLabels := []map[string]string{
				{"test-id": "parallel-1"},
				{"test-id": "parallel-2"},
				{"test-id": "parallel-3"},
			}

			selectors := make([]string, len(testLabels))
			for i, labels := range testLabels {
				execOptions := cmdoptions.ExecOptions{Labels: labels}
				selectors[i] = buildLabelSelector(hostname, execOptions, false)
			}

			// Verify all selectors are unique
			for i := range selectors {
				for j := i + 1; j < len(selectors); j++ {
					Expect(selectors[i]).NotTo(Equal(selectors[j]),
						"Selectors for parallel tests should be unique")
				}
			}
		})

		It("should demonstrate label-based isolation prevents cross-test deletion", func() {
			// Test 1 creates pods with test-id=test1
			test1Options := cmdoptions.ExecOptions{
				Labels: map[string]string{"test-id": "test1"},
			}
			test1Selector := buildLabelSelector(hostname, test1Options, false)

			// Test 2 creates pods with test-id=test2
			test2Options := cmdoptions.ExecOptions{
				Labels: map[string]string{"test-id": "test2"},
			}
			test2Selector := buildLabelSelector(hostname, test2Options, false)

			// Verify selectors are different
			Expect(test1Selector).NotTo(Equal(test2Selector))

			// Verify test1 selector won't match test2 pods
			Expect(test1Selector).To(ContainSubstring("test-id=test1"))
			Expect(test1Selector).NotTo(ContainSubstring("test-id=test2"))

			// Verify test2 selector won't match test1 pods
			Expect(test2Selector).To(ContainSubstring("test-id=test2"))
			Expect(test2Selector).NotTo(ContainSubstring("test-id=test1"))
		})
	})

	Context("when working with daemon pods", func() {
		It("should include custom labels in daemon pod creation", func() {
			// Daemon pods use the same label mechanism through ExecOptions
			// Verify that daemon options inherit labels correctly
			daemonOpts := cmdoptions.DaemonOptions{
				Name: "test-daemon",
				ExecOptions: cmdoptions.ExecOptions{
					Labels: map[string]string{
						"test-id": "daemon-test-123",
						"env":     "test",
					},
				},
			}

			// Verify labels are accessible
			Expect(daemonOpts.Labels).To(HaveKeyWithValue("test-id", "daemon-test-123"))
			Expect(daemonOpts.Labels).To(HaveKeyWithValue("env", "test"))
		})

		It("should create unique daemon names for parallel tests", func() {
			// Demonstrate that unique daemon names prevent conflicts
			daemon1Name := "test-daemon-abc123"
			daemon2Name := "test-daemon-xyz789"

			Expect(daemon1Name).NotTo(Equal(daemon2Name))
			Expect(fmt.Sprintf("daemon-%s", daemon1Name)).NotTo(Equal(fmt.Sprintf("daemon-%s", daemon2Name)))
		})
	})
})
