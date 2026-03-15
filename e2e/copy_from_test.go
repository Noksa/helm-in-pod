//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Copy From Pod", func() {
	var (
		testNS    string
		testLabel string
		tmpDir    string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-copyfrom")
		testLabel = generateTestLabel()

		var err error
		tmpDir, err = os.MkdirTemp("", "hip-copyfrom-*")
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
		_ = os.RemoveAll(tmpDir)
	})

	Context("exec mode", func() {
		It("should copy a single file from pod to host", func() {
			hostPath := filepath.Join(tmpDir, "output")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/result.txt:%s", hostPath),
				"--",
				"sh -c 'echo hello-from-pod > /tmp/result.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(filepath.Join(hostPath, "result.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("hello-from-pod"))
		})

		It("should copy a directory from pod to host", func() {
			hostPath := filepath.Join(tmpDir, "outdir")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/mydir:%s", hostPath),
				"--",
				"sh -c 'mkdir -p /tmp/mydir && echo file-a > /tmp/mydir/a.txt && echo file-b > /tmp/mydir/b.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			dataA, err := os.ReadFile(filepath.Join(hostPath, "mydir", "a.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataA)).To(ContainSubstring("file-a"))

			dataB, err := os.ReadFile(filepath.Join(hostPath, "mydir", "b.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataB)).To(ContainSubstring("file-b"))
		})

		It("should copy multiple files with multiple --copy-from flags", func() {
			hostPath1 := filepath.Join(tmpDir, "out1")
			hostPath2 := filepath.Join(tmpDir, "out2")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/f1.txt:%s", hostPath1),
				"--copy-from", fmt.Sprintf("/tmp/f2.txt:%s", hostPath2),
				"--",
				"sh -c 'echo first > /tmp/f1.txt && echo second > /tmp/f2.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data1, err := os.ReadFile(filepath.Join(hostPath1, "f1.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data1)).To(ContainSubstring("first"))

			data2, err := os.ReadFile(filepath.Join(hostPath2, "f2.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data2)).To(ContainSubstring("second"))
		})

		It("should still copy files even when command fails", func() {
			hostPath := filepath.Join(tmpDir, "failout")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/artifact.txt:%s", hostPath),
				"--",
				"sh -c 'echo artifact-data > /tmp/artifact.txt && exit 1'",
			)
			output, exitCode := RunWithExitCode(cmd)
			// Command should fail
			Expect(exitCode).NotTo(Equal(0), "output: %s", output)

			// But the file should still be copied
			data, err := os.ReadFile(filepath.Join(hostPath, "artifact.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("artifact-data"))
		})
	})

	Context("daemon mode", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("copyfrom-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should copy file from daemon pod to host", func() {
			hostPath := filepath.Join(tmpDir, "daemon-out")
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--copy-from", fmt.Sprintf("/tmp/daemon-result.txt:%s", hostPath),
				"--", "sh -c 'echo daemon-output > /tmp/daemon-result.txt'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(filepath.Join(hostPath, "daemon-result.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("daemon-output"))
		})
	})
})
