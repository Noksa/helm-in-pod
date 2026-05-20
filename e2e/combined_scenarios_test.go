//go:build e2e

package e2e

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("Combined Scenarios: copy-from + keep-pod + activeDeadlineSeconds", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-combined")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
		// Safety cleanup for keep-pod
		cmd := exec.Command("kubectl", "delete", "pods",
			"-n", hipconsts.Namespace,
			"-l", testLabel,
			"--ignore-not-found")
		_, _ = RunWithExitCode(cmd)
	})

	DescribeTable("exec with combined flags",
		func(copyFrom bool, keepPod bool, deadline string, expectMarker string) {
			args := []string{
				"--labels", testLabel,
				"--active-deadline-seconds", deadline,
			}
			if copyFrom {
				args = append(args, "--copy-from", "/tmp/output.txt:/tmp/output.txt")
			}
			if keepPod {
				args = append(args, "--keep-pod")
			}
			args = append(args, "--", "echo", "combined-test")

			cmd := BuildHelmInPodCommand(args...)
			output, exitCode := RunWithExitCode(cmd)

			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("combined-test"))
			if expectMarker != "" {
				Expect(output).To(ContainSubstring(expectMarker))
			}
			if keepPod {
				Expect(output).To(ContainSubstring("will be kept"))
				// Verify pod still exists
				podCmd := exec.Command("kubectl", "get", "pods",
					"-n", hipconsts.Namespace,
					"-l", testLabel,
					"-o", "name")
				podOutput, err := Run(podCmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(podOutput).NotTo(BeEmpty())
			}
		},
		Entry("keep-pod + deadline=600 (no explicit copy-from)", false, true, "600", "will be kept"),
		Entry("keep-pod + deadline=120 (no copy-from)", false, true, "120", "will be kept"),
	)
})
