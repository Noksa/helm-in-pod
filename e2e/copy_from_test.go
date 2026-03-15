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
		It("should copy a single file from pod to host as a file", func() {
			hostPath := filepath.Join(tmpDir, "result.txt")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/result.txt:%s", hostPath),
				"--",
				"sh -c 'echo hello-from-pod > /tmp/result.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			info, err := os.Stat(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeFalse(), "expected a file, got a directory")

			data, err := os.ReadFile(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("hello-from-pod"))
		})

		It("should copy a directory contents directly into host path", func() {
			hostPath := filepath.Join(tmpDir, "outdir")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/mydir:%s", hostPath),
				"--",
				"sh -c 'mkdir -p /tmp/mydir && echo file-a > /tmp/mydir/a.txt && echo file-b > /tmp/mydir/b.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			// Contents should be directly inside hostPath, not nested under mydir/
			dataA, err := os.ReadFile(filepath.Join(hostPath, "a.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataA)).To(ContainSubstring("file-a"))

			dataB, err := os.ReadFile(filepath.Join(hostPath, "b.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataB)).To(ContainSubstring("file-b"))

			// The extra nested directory should NOT exist
			Expect(filepath.Join(hostPath, "mydir")).NotTo(BeADirectory())
		})

		It("should copy multiple files with multiple --copy-from flags", func() {
			hostPath1 := filepath.Join(tmpDir, "f1.txt")
			hostPath2 := filepath.Join(tmpDir, "f2.txt")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/f1.txt:%s", hostPath1),
				"--copy-from", fmt.Sprintf("/tmp/f2.txt:%s", hostPath2),
				"--",
				"sh -c 'echo first > /tmp/f1.txt && echo second > /tmp/f2.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data1, err := os.ReadFile(hostPath1)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data1)).To(ContainSubstring("first"))

			data2, err := os.ReadFile(hostPath2)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data2)).To(ContainSubstring("second"))
		})

		It("should still copy files even when command fails", func() {
			hostPath := filepath.Join(tmpDir, "artifact.txt")
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
			data, err := os.ReadFile(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("artifact-data"))
		})

		It("should copy a file with a different destination name", func() {
			hostPath := filepath.Join(tmpDir, "renamed-output.txt")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/original.txt:%s", hostPath),
				"--",
				"sh -c 'echo renamed-content > /tmp/original.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("renamed-content"))
		})

		It("should copy a nested directory without extra nesting", func() {
			hostPath := filepath.Join(tmpDir, "nested", "dest")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/srcdir:%s", hostPath),
				"--",
				"sh -c 'mkdir -p /tmp/srcdir/sub && echo deep > /tmp/srcdir/sub/deep.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(filepath.Join(hostPath, "sub", "deep.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("deep"))

			// Should NOT have extra srcdir/ level
			Expect(filepath.Join(hostPath, "srcdir")).NotTo(BeADirectory())
		})

		It("should copy a file into an existing directory when hostPath is a dir", func() {
			// Pre-create the target directory
			destDir := filepath.Join(tmpDir, "existing-dir")
			Expect(os.MkdirAll(destDir, 0o755)).To(Succeed())

			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/into-dir.txt:%s", destDir),
				"--",
				"sh -c 'echo into-dir-content > /tmp/into-dir.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			// File should be placed inside the directory with its original name
			data, err := os.ReadFile(filepath.Join(destDir, "into-dir.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("into-dir-content"))
		})

		It("should copy a directory into current dir with dot host path", func() {
			// Use tmpDir as working directory equivalent
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/dotdir:%s", tmpDir),
				"--",
				"sh -c 'mkdir -p /tmp/dotdir && echo dot-content > /tmp/dotdir/dot.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(filepath.Join(tmpDir, "dot.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("dot-content"))
		})

		It("should copy a file to a deep non-existent path", func() {
			hostPath := filepath.Join(tmpDir, "a", "b", "c", "result.txt")
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--copy-from", fmt.Sprintf("/tmp/deep.txt:%s", hostPath),
				"--",
				"sh -c 'echo deep-file > /tmp/deep.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("deep-file"))
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
			hostPath := filepath.Join(tmpDir, "daemon-result.txt")
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--copy-from", fmt.Sprintf("/tmp/daemon-result.txt:%s", hostPath),
				"--", "sh -c 'echo daemon-output > /tmp/daemon-result.txt'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)

			data, err := os.ReadFile(hostPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("daemon-output"))
		})
	})
})
