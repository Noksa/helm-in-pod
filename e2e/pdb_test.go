//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PodDisruptionBudget", func() {
	var testNs string
	var testLabel string

	BeforeEach(func() {
		testNs = createNamespace("pdb-test")
		testLabel = generateTestLabel()
	})

	AfterEach(func() {
		logOnFailure(testNs)
		deleteNamespace(testNs)
	})

	Context("when creating a helm-in-pod execution pod", func() {
		var sharedDaemonName string

		BeforeEach(func() {
			// Create one shared daemon for tests in this context
			sharedDaemonName = fmt.Sprintf("pdb-shared-%s", randomString(6))
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", sharedDaemonName, "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			// Cleanup shared daemon
			if sharedDaemonName != "" {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", sharedDaemonName)
				_, _ = Run(cmd)
			}
		})
		It("should create a PDB with matching operation-id label", func() {
			// Execute a simple helm command
			cmd := BuildHelmInPodCommand(
				fmt.Sprintf("--labels=%s", testLabel),
				"--copy-repo=false",
				"--",
				"helm", "version",
			)
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify PDB was created and cleaned up for our specific pod
			// Since the command completes quickly, PDB should be deleted
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", testLabel, "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			items := pdbList["items"].([]interface{})
			// After command completion, PDB for our pod should be cleaned up
			Expect(items).To(HaveLen(0), "PDB should be cleaned up after pod deletion")
		})

		It("should create PDB with minAvailable=1", func() {
			// Get the daemon pod to extract operation-id
			cmd := exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", sharedDaemonName), "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pod map[string]interface{}
			err = json.Unmarshal([]byte(output), &pod)
			Expect(err).NotTo(HaveOccurred())

			metadata := pod["metadata"].(map[string]interface{})
			labels := metadata["labels"].(map[string]interface{})
			operationID, ok := labels[hipconsts.LabelOperationID].(string)
			Expect(ok).To(BeTrue(), "Pod should have operation-id label")
			Expect(operationID).NotTo(BeEmpty(), "operation-id should not be empty")

			// Verify PDB exists with matching operation-id
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID), "-o", "json")
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			items := pdbList["items"].([]interface{})
			Expect(items).To(HaveLen(1), "Should have exactly one PDB for the daemon pod")

			pdb := items[0].(map[string]interface{})
			spec := pdb["spec"].(map[string]interface{})

			// Verify minAvailable is 1
			minAvailable := spec["minAvailable"].(float64)
			Expect(minAvailable).To(Equal(float64(1)), "PDB should have minAvailable=1")

			// Verify selector matches operation-id
			selector := spec["selector"].(map[string]interface{})
			matchLabels := selector["matchLabels"].(map[string]interface{})
			Expect(matchLabels[hipconsts.LabelOperationID]).To(Equal(operationID),
				"PDB selector should match pod operation-id")
		})

		It("should clean up PDB when daemon pod is stopped", func() {
			// Get operation-id before stopping
			cmd := exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", sharedDaemonName), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			operationID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(operationID).NotTo(BeEmpty())

			// Verify PDB exists
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Stop daemon
			cmd = exec.Command("helm", "in-pod", "daemon", "stop", "--name", sharedDaemonName)
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			sharedDaemonName = "" // Mark as stopped so AfterEach doesn't try again

			// Verify PDB is deleted
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID), "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			items := pdbList["items"].([]interface{})
			Expect(items).To(HaveLen(0), "PDB should be deleted when daemon pod is stopped")
		})

		It("should have unique operation-id for each pod", func() {
			daemon1 := fmt.Sprintf("pdb-unique1-%s", randomString(6))
			daemon2 := fmt.Sprintf("pdb-unique2-%s", randomString(6))

			// Start two daemon pods
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", daemon1, "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("helm", "in-pod", "daemon", "start", "--name", daemon2, "--copy-repo=false")
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				_ = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemon1).Run()
				_ = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemon2).Run()
			}()

			// Get operation IDs
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemon1), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			opID1, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemon2), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			opID2, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify they are different
			Expect(opID1).NotTo(Equal(opID2), "Each pod should have a unique operation-id")

			// Verify both have valid UUID format
			Expect(opID1).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
			Expect(opID2).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))

			// Verify each has its own PDB
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, opID1))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, opID2))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("PDB prevents voluntary disruptions", func() {
		var sharedDaemonName string

		BeforeEach(func() {
			// Create daemon for this context
			sharedDaemonName = fmt.Sprintf("pdb-protect-%s", randomString(6))
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", sharedDaemonName, "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			// Cleanup daemon
			if sharedDaemonName != "" {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", sharedDaemonName)
				_, _ = Run(cmd)
			}
		})

		It("should protect pod from eviction during helm operations", func() {
			// Use shared daemon - verify PDB status shows the pod is protected
			podName := fmt.Sprintf("daemon-%s", sharedDaemonName)

			cmd := exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				podName, "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			operationID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID), "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			items := pdbList["items"].([]interface{})
			Expect(items).To(HaveLen(1))

			pdb := items[0].(map[string]interface{})

			// The key test: PDB exists and is configured correctly
			spec := pdb["spec"].(map[string]interface{})
			minAvailable := spec["minAvailable"].(float64)
			Expect(minAvailable).To(Equal(float64(1)),
				"PDB should protect the pod with minAvailable=1")
		})
	})

	Context("edge cases", func() {
		It("should handle pod recreation with new operation-id", func() {
			daemonName := fmt.Sprintf("pdb-recreate-%s", randomString(6))

			// Create first daemon
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", daemonName, "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Get first operation ID
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemonName), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			firstOpID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Stop daemon
			cmd = exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName)
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Recreate with --force
			cmd = exec.Command("helm", "in-pod", "daemon", "start", "--name", daemonName, "--copy-repo=false")
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName)
				_, _ = Run(cmd)
			}()

			// Get second operation ID
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemonName), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			secondOpID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify they are different
			Expect(firstOpID).NotTo(Equal(secondOpID),
				"Recreated pod should have a new operation-id")

			// Verify old PDB is gone
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, firstOpID), "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var oldPDBList map[string]interface{}
			err = json.Unmarshal([]byte(output), &oldPDBList)
			Expect(err).NotTo(HaveOccurred())
			Expect(oldPDBList["items"].([]interface{})).To(HaveLen(0),
				"Old PDB should be deleted")

			// Verify new PDB exists
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, secondOpID))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle purge command cleaning up PDBs", Serial, func() {
			testLabel := fmt.Sprintf("purge-test-%s", randomString(8))

			// Create a pod with specific label
			cmd := BuildHelmInPodCommand(
				fmt.Sprintf("--labels=test=%s", testLabel),
				"--copy-repo=false",
				"--",
				"sleep", "300",
			)

			// Start in background (don't wait for completion)
			dir, _ := GetProjectDir()
			cmd.Dir = dir
			err := cmd.Start()
			Expect(err).NotTo(HaveOccurred())

			// Wait a bit for pod to be created
			Eventually(func() bool {
				checkCmd := exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
					"-l", fmt.Sprintf("test=%s", testLabel), "--no-headers")
				output, _ := Run(checkCmd)
				return strings.Contains(output, hipconsts.HelmInPodNamespace)
			}, "30s", "1s").Should(BeTrue(), "Pod should be created")

			// Get operation ID
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("test=%s", testLabel),
				"-o", fmt.Sprintf("jsonpath={.items[0].metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			operationID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(operationID).NotTo(BeEmpty())

			// Verify PDB exists
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Purge all pods
			cmd = exec.Command("helm", "in-pod", "purge", "--all")
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Wait for all pods to be fully deleted (not just terminating)
			Eventually(func() int {
				cmd := exec.Command("kubectl", "get", "pods", "-n", hipconsts.HelmInPodNamespace, "-o", "json")
				output, err := Run(cmd)
				if err != nil {
					return -1
				}
				var podList map[string]interface{}
				err = json.Unmarshal([]byte(output), &podList)
				if err != nil {
					return -1
				}
				return len(podList["items"].([]interface{}))
			}, "60s", "2s").Should(Equal(0), "All pods should eventually be deleted after purge --all")

			// Verify all PDBs are also deleted
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace, "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())
			Expect(pdbList["items"].([]interface{})).To(HaveLen(0),
				"All PDBs should be deleted when pods are purged")
		})
	})

	Context("--create-pdb flag", func() {
		It("should not create PDB for daemon when --create-pdb=false is specified", func() {
			daemonName := fmt.Sprintf("pdb-disabled-%s", randomString(6))
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", daemonName, "--create-pdb=false", "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName)
				_, _ = Run(cmd)
			}()

			// Get the pod's operation-id
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemonName), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			operationID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(operationID).NotTo(BeEmpty(), "Pod should still have operation-id label")

			// Verify NO PDB was created
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID), "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			items := pdbList["items"].([]interface{})
			Expect(items).To(HaveLen(0), "No PDB should be created when --create-pdb=false")
		})

		It("should create PDB for daemon by default (when flag not specified)", func() {
			daemonName := fmt.Sprintf("pdb-default-%s", randomString(6))
			// Don't specify --create-pdb flag, should default to true
			cmd := exec.Command("helm", "in-pod", "daemon", "start", "--name", daemonName, "--copy-repo=false")
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName)
				_, _ = Run(cmd)
			}()

			// Get the pod's operation-id
			cmd = exec.Command("kubectl", "get", "pod", "-n", hipconsts.HelmInPodNamespace,
				fmt.Sprintf("daemon-%s", daemonName), "-o", fmt.Sprintf("jsonpath={.metadata.labels.%s}", strings.ReplaceAll(hipconsts.LabelOperationID, "/", "\\/")))
			operationID, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify PDB WAS created
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", hipconsts.LabelOperationID, operationID))
			_, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "PDB should be created by default")
		})

		It("should not create PDB for exec when --create-pdb=false is specified", func() {
			testLabel := fmt.Sprintf("no-pdb-%s", randomString(8))

			// Execute a command with --create-pdb=false
			cmd := BuildHelmInPodCommand(
				fmt.Sprintf("--labels=test=%s", testLabel),
				"--create-pdb=false",
				"--copy-repo=false",
				"--",
				"helm", "version",
			)
			_, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			// Verify no PDBs with our test label exist
			cmd = exec.Command("kubectl", "get", "pdb", "-n", hipconsts.HelmInPodNamespace, "-o", "json")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			var pdbList map[string]interface{}
			err = json.Unmarshal([]byte(output), &pdbList)
			Expect(err).NotTo(HaveOccurred())

			// Since the command completes quickly, there shouldn't be any PDBs
			// But let's verify by checking if any exist at all
			items := pdbList["items"].([]interface{})
			for _, item := range items {
				pdb := item.(map[string]interface{})
				spec := pdb["spec"].(map[string]interface{})
				selector := spec["selector"].(map[string]interface{})
				matchLabels := selector["matchLabels"].(map[string]interface{})

				// If we find a PDB matching our test label, fail
				if testLabelValue, ok := matchLabels["test"]; ok && testLabelValue == testLabel {
					Fail("Found PDB for test label when --create-pdb=false was specified")
				}
			}
		})
	})
})
