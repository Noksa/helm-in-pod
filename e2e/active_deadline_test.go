//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

var _ = Describe("Active Deadline Seconds Flag", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-deadline")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	// ─────────────────────────────────────────────────────────────────────────
	// DRY-RUN: spec generation
	// ─────────────────────────────────────────────────────────────────────────

	Context("--dry-run spec generation", func() {
		It("should include activeDeadlineSeconds: 1800 in exec pod spec", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "1800",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 1800"))
		})

		It("should include activeDeadlineSeconds: 3600 in daemon pod spec", func() {
			daemonName := generateReleaseName("deadline-daemon")
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "3600",
				"-n", testNS,
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 3600"))
		})

		It("should include activeDeadlineSeconds: 86400 (1 day) in pod spec", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "86400",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 86400"))
		})

		It("should include activeDeadlineSeconds: 60 in pod spec", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "60",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("activeDeadlineSeconds: 60"))
		})

		It("should NOT include activeDeadlineSeconds when flag is not passed", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).NotTo(ContainSubstring("activeDeadlineSeconds"),
				"activeDeadlineSeconds must be absent when not set")
		})

		It("should NOT include activeDeadlineSeconds when flag is explicitly 0", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "0",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).NotTo(ContainSubstring("activeDeadlineSeconds"),
				"activeDeadlineSeconds: 0 means no limit — must not appear in spec")
		})

		It("should not create any real pod during dry-run with active-deadline-seconds", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--dry-run",
				"--active-deadline-seconds", "60",
				"--", "helm version",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			// parse label key=value for kubectl
			parts := strings.SplitN(testLabel, "=", 2)
			Expect(parts).To(HaveLen(2))
			cmd = exec.Command("kubectl", "get", "pods",
				"-n", hipconsts.HelmInPodNamespace,
				"-l", fmt.Sprintf("%s=%s", parts[0], parts[1]),
				"-o", "name")
			podOutput, _ := Run(cmd)
			Expect(strings.TrimSpace(podOutput)).To(BeEmpty(),
				"dry-run must not create any pod in the cluster")
		})
	})

	// ─────────────────────────────────────────────────────────────────────────
	// REAL POD: field verification in Kubernetes
	// ─────────────────────────────────────────────────────────────────────────

	Context("real pod field verification", func() {
		// safeDeadlineQuery returns the activeDeadlineSeconds value for the first pod
		// matching testLabel in HelmInPodNamespace, or "" if no pod exists yet.
		// Uses range jsonpath so kubectl exits 0 even on empty lists.
		safeDeadlineQuery := func(lbl string) func() string {
			return func() string {
				parts := strings.SplitN(lbl, "=", 2)
				if len(parts) != 2 {
					return ""
				}
				cmd := exec.Command("kubectl", "get", "pods",
					"-n", hipconsts.HelmInPodNamespace,
					"-l", fmt.Sprintf("%s=%s", parts[0], parts[1]),
					"-o", "jsonpath={range .items[*]}{.spec.activeDeadlineSeconds}{end}")
				out, _ := RunWithExitCode(cmd)
				return strings.TrimSpace(out)
			}
		}

		It("should set activeDeadlineSeconds in the running pod object", func() {
			// Run in background so we can inspect the pod while it's alive
			done := make(chan struct{})
			var cmdOutput string
			var cmdExit int
			go func() {
				defer close(done)
				cmd := BuildHelmInPodCommand(
					"--labels", testLabel,
					"--active-deadline-seconds", "120",
					"--", "sleep 30",
				)
				cmdOutput, cmdExit = RunWithExitCode(cmd)
			}()

			// Poll until the pod appears with the expected field (up to 45s)
			Eventually(safeDeadlineQuery(testLabel),
				45*time.Second, 3*time.Second,
			).Should(Equal("120"),
				"activeDeadlineSeconds must be 120 on the live pod object")

			<-done
			Expect(cmdExit).To(Equal(0), "pod should complete before 120s deadline, output: %s", cmdOutput)
		})

		It("should have no activeDeadlineSeconds field when flag is not set", func() {
			done := make(chan struct{})
			go func() {
				defer close(done)
				cmd := BuildHelmInPodCommand(
					"--labels", testLabel,
					"--", "sleep 20",
				)
				_, _ = RunWithExitCode(cmd)
			}()

			// Wait for the pod to be scheduled, then verify the field is absent
			time.Sleep(18 * time.Second)
			Expect(safeDeadlineQuery(testLabel)()).To(BeEmpty(),
				"activeDeadlineSeconds must be absent when flag is not set")

			<-done
		})
	})

	// ─────────────────────────────────────────────────────────────────────────
	// RUNTIME ENFORCEMENT
	// ─────────────────────────────────────────────────────────────────────────

	Context("runtime enforcement", func() {
		It("should complete successfully when command finishes well before the deadline", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "300",
				"--", "echo deadline-ok",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("deadline-ok"))
		})

		It("should preserve full stdout output when command completes before deadline", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "300",
				"--", "echo line1; echo line2; echo line3",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("line1"))
			Expect(output).To(ContainSubstring("line2"))
			Expect(output).To(ContainSubstring("line3"))
		})

		It("should propagate non-zero exit code when command itself fails before deadline", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "300",
				"--", "exit 42",
			)
			_, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(42),
				"exit code from the command must be preserved")
		})

		It("should terminate pod when active deadline expires before command completes", func(_ context.Context) {
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--active-deadline-seconds", "5",
				"--timeout", "2m",
				"--", "sleep 300",
			)
			output, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).NotTo(Equal(0),
				"pod must be terminated by Kubernetes deadline, output: %s", output)
			Expect(elapsed).To(BeNumerically("<", 3*time.Minute),
				"should terminate well before sleep 300 finishes")
		}, NodeTimeout(5*time.Minute))

		It("should terminate faster with short deadline than with a long timeout", func(_ context.Context) {
			// Baseline: same command but large deadline → must run longer
			longStart := time.Now()
			cmdLong := BuildHelmInPodCommand(
				"--labels", generateTestLabel(),
				"--active-deadline-seconds", "300",
				"--timeout", "5m",
				"--", "sleep 15",
			)
			_, longExit := RunWithExitCode(cmdLong)
			longElapsed := time.Since(longStart)

			// Short deadline: pod must be killed sooner
			shortStart := time.Now()
			cmdShort := BuildHelmInPodCommand(
				"--labels", generateTestLabel(),
				"--active-deadline-seconds", "5",
				"--timeout", "2m",
				"--", "sleep 300",
			)
			_, shortExit := RunWithExitCode(cmdShort)
			shortElapsed := time.Since(shortStart)

			Expect(longExit).To(Equal(0), "long-deadline command should complete")
			Expect(shortExit).NotTo(Equal(0), "short-deadline command must be killed")
			Expect(shortElapsed).To(BeNumerically("<", longElapsed+60*time.Second),
				"short-deadline pod should be killed quickly")
		}, NodeTimeout(8*time.Minute))
	})

	// ─────────────────────────────────────────────────────────────────────────
	// CONCURRENT EXECUTION: isolation
	// ─────────────────────────────────────────────────────────────────────────

	Context("concurrent executions with different deadlines", func() {
		It("should handle multiple concurrent exec pods with independent deadlines", func(_ context.Context) {
			type result struct {
				label    string
				exitCode int
				output   string
				elapsed  time.Duration
			}

			var mu sync.Mutex
			var wg sync.WaitGroup
			results := make([]result, 3)

			// Pod 0: short deadline (dies)
			wg.Add(1)
			go func() {
				defer wg.Done()
				label := generateTestLabel()
				start := time.Now()
				cmd := BuildHelmInPodCommand(
					"--labels", label,
					"--active-deadline-seconds", "5",
					"--timeout", "2m",
					"--", "sleep 300",
				)
				out, code := RunWithExitCode(cmd)
				mu.Lock()
				results[0] = result{label, code, out, time.Since(start)}
				mu.Unlock()
			}()

			// Pod 1: generous deadline (completes)
			wg.Add(1)
			go func() {
				defer wg.Done()
				label := generateTestLabel()
				start := time.Now()
				cmd := BuildHelmInPodCommand(
					"--labels", label,
					"--active-deadline-seconds", "300",
					"--", "echo concurrent-ok",
				)
				out, code := RunWithExitCode(cmd)
				mu.Lock()
				results[1] = result{label, code, out, time.Since(start)}
				mu.Unlock()
			}()

			// Pod 2: no deadline (completes normally)
			wg.Add(1)
			go func() {
				defer wg.Done()
				label := generateTestLabel()
				start := time.Now()
				cmd := BuildHelmInPodCommand(
					"--labels", label,
					"--", "echo no-deadline-ok",
				)
				out, code := RunWithExitCode(cmd)
				mu.Lock()
				results[2] = result{label, code, out, time.Since(start)}
				mu.Unlock()
			}()

			wg.Wait()

			// Pod 0: killed by deadline
			Expect(results[0].exitCode).NotTo(Equal(0),
				"short-deadline pod must be killed, output: %s", results[0].output)
			Expect(results[0].elapsed).To(BeNumerically("<", 3*time.Minute),
				"short-deadline pod must die well before sleep 300")

			// Pod 1: completes with output
			Expect(results[1].exitCode).To(Equal(0),
				"generous-deadline pod must succeed, output: %s", results[1].output)
			Expect(results[1].output).To(ContainSubstring("concurrent-ok"))

			// Pod 2: no deadline, completes normally
			Expect(results[2].exitCode).To(Equal(0),
				"no-deadline pod must succeed, output: %s", results[2].output)
			Expect(results[2].output).To(ContainSubstring("no-deadline-ok"))
		}, NodeTimeout(8*time.Minute))
	})

	// ─────────────────────────────────────────────────────────────────────────
	// DAEMON POD: real creation
	// ─────────────────────────────────────────────────────────────────────────

	Context("daemon pod with active-deadline-seconds", func() {
		var daemonName string

		AfterEach(func() {
			if daemonName != "" {
				cmd := exec.Command("helm", "in-pod", "daemon", "stop",
					"--name", daemonName, "-n", testNS)
				_, _ = Run(cmd)
			}
		})

		It("should start a daemon pod and expose activeDeadlineSeconds in its live spec", func() {
			daemonName = generateReleaseName("dl-daemon")
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--active-deadline-seconds", "600",
				"-n", testNS,
			)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "daemon start failed: %s", output)

			// Daemon pods always live in the helm-in-pod namespace, named "daemon-<name>"
			podName := fmt.Sprintf("daemon-%s", daemonName)
			cmd = exec.Command("kubectl", "get", "pod", podName,
				"-n", hipconsts.HelmInPodNamespace,
				"-o", "jsonpath={.spec.activeDeadlineSeconds}")
			podOutput, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(),
				"failed to query daemon pod %s in ns %s", podName, hipconsts.HelmInPodNamespace)
			Expect(strings.TrimSpace(podOutput)).To(Equal("600"),
				"daemon pod must have activeDeadlineSeconds=600 in its live spec")
		})

		It("should allow daemon exec to complete successfully before the daemon deadline", func() {
			daemonName = generateReleaseName("dl-exec-daemon")
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--active-deadline-seconds", "300",
				"-n", testNS,
			)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "daemon start failed: %s", output)

			cmd = exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "echo daemon-deadline-exec-ok")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("daemon-deadline-exec-ok"))
		})

		It("should start a daemon pod without activeDeadlineSeconds when flag is not set", func() {
			daemonName = generateReleaseName("dl-nodl-daemon")
			cmd := BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"-n", testNS,
			)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "daemon start failed: %s", output)

			// Daemon pods always live in the helm-in-pod namespace, named "daemon-<name>"
			// Field absent → empty string returned, kubectl exits 0 for a specific pod
			podName := fmt.Sprintf("daemon-%s", daemonName)
			cmd = exec.Command("kubectl", "get", "pod", podName,
				"-n", hipconsts.HelmInPodNamespace,
				"-o", "jsonpath={.spec.activeDeadlineSeconds}")
			podOutput, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(),
				"failed to query daemon pod %s in ns %s", podName, hipconsts.HelmInPodNamespace)
			Expect(strings.TrimSpace(podOutput)).To(BeEmpty(),
				"daemon pod must NOT have activeDeadlineSeconds when flag is not set")
		})
	})

	// ─────────────────────────────────────────────────────────────────────────
	// INTERACTION: combined flags
	// ─────────────────────────────────────────────────────────────────────────

	Context("interaction with other flags", func() {
		It("should respect --active-deadline-seconds alongside custom --labels", func() {
			extraLabel := generateTestLabel()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--labels", extraLabel,
				"--active-deadline-seconds", "300",
				"--", "echo labels-and-deadline-ok",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("labels-and-deadline-ok"))
		})

		It("should apply activeDeadlineSeconds to each exec pod independently when run sequentially", func() {
			for i := 0; i < 3; i++ {
				lbl := generateTestLabel()
				cmd := BuildHelmInPodCommand(
					"--labels", lbl,
					"--active-deadline-seconds", "300",
					"--", fmt.Sprintf("echo seq-run-%d", i),
				)
				output, exitCode := RunWithExitCode(cmd)
				Expect(exitCode).To(Equal(0), "run %d failed, output: %s", i, output)
				Expect(output).To(ContainSubstring(fmt.Sprintf("seq-run-%d", i)))
			}
		})

		It("should coexist correctly with --timeout when deadline is longer than timeout", func(_ context.Context) {
			// timeout=10s, deadline=120s → timeout fires first
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--timeout", "10s",
				"--active-deadline-seconds", "120",
				"--", "sleep 300",
			)
			_, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).NotTo(Equal(0), "should fail due to --timeout")
			Expect(elapsed).To(BeNumerically("<", 3*time.Minute),
				"--timeout should fire before the longer deadline")
		}, NodeTimeout(5*time.Minute))

		It("should coexist correctly with --timeout when deadline is shorter than timeout", func(_ context.Context) {
			// deadline=5s, timeout=2m → deadline fires first
			start := time.Now()
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--timeout", "2m",
				"--active-deadline-seconds", "5",
				"--", "sleep 300",
			)
			_, exitCode := RunWithExitCode(cmd)
			elapsed := time.Since(start)

			Expect(exitCode).NotTo(Equal(0), "should fail due to deadline")
			Expect(elapsed).To(BeNumerically("<", 2*time.Minute),
				"deadline should fire before the longer --timeout")
		}, NodeTimeout(5*time.Minute))
	})
})
