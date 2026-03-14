//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Copy Flag", func() {
	var (
		testNS    string
		testLabel string
		tmpDir    string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-copy")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })

		var err error
		tmpDir, err = os.MkdirTemp("", "hip-copy-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		logOnFailure(testNS)
		_ = os.RemoveAll(tmpDir)
	})

	Context("single file copy", func() {
		It("should copy a single file to the pod", func() {
			filePath := filepath.Join(tmpDir, "test.txt")
			Expect(os.WriteFile(filePath, []byte("hello from host"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/test.txt", filePath),
				"--", "cat /tmp/test.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("hello from host"))
		})

		It("should copy a file with spaces in content", func() {
			filePath := filepath.Join(tmpDir, "spaces.txt")
			Expect(os.WriteFile(filePath, []byte("line 1\nline 2\nline 3"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/spaces.txt", filePath),
				"--", "wc -l /tmp/spaces.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("3"))
		})
	})

	Context("directory copy", func() {
		It("should copy an entire directory to the pod", func() {
			subDir := filepath.Join(tmpDir, "mydir")
			Expect(os.MkdirAll(subDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(subDir, "a.txt"), []byte("file-a"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("file-b"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/mydir", subDir),
				"--", "sh -c 'cat /tmp/mydir/a.txt && echo --- && cat /tmp/mydir/b.txt'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("file-a"))
			Expect(output).To(ContainSubstring("file-b"))
		})

		It("should copy nested directory structure", func() {
			nested := filepath.Join(tmpDir, "parent", "child")
			Expect(os.MkdirAll(nested, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("deep-content"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/parent", filepath.Join(tmpDir, "parent")),
				"--", "cat /tmp/parent/child/deep.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("deep-content"))
		})
	})

	Context("multiple file copies", func() {
		It("should copy multiple files with multiple --copy flags", func() {
			file1 := filepath.Join(tmpDir, "one.txt")
			file2 := filepath.Join(tmpDir, "two.txt")
			Expect(os.WriteFile(file1, []byte("first"), 0644)).To(Succeed())
			Expect(os.WriteFile(file2, []byte("second"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/one.txt", file1),
				"--copy", fmt.Sprintf("%s:/tmp/two.txt", file2),
				"--", "sh -c 'cat /tmp/one.txt && echo --- && cat /tmp/two.txt'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("first"))
			Expect(output).To(ContainSubstring("second"))
		})

		It("should copy multiple files with comma-separated syntax", func() {
			file1 := filepath.Join(tmpDir, "alpha.txt")
			file2 := filepath.Join(tmpDir, "beta.txt")
			Expect(os.WriteFile(file1, []byte("alpha-content"), 0644)).To(Succeed())
			Expect(os.WriteFile(file2, []byte("beta-content"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/alpha.txt,%s:/tmp/beta.txt", file1, file2),
				"--", "sh -c 'cat /tmp/alpha.txt && echo --- && cat /tmp/beta.txt'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("alpha-content"))
			Expect(output).To(ContainSubstring("beta-content"))
		})
	})

	Context("copy in daemon mode", func() {
		var daemonName string

		BeforeEach(func() {
			daemonName = fmt.Sprintf("copy-daemon-%s", randomString(6))
			cmd := BuildDaemonStartCommand("--name", daemonName, "--labels", testLabel, "-n", testNS)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
		})

		It("should copy files via daemon exec", func() {
			filePath := filepath.Join(tmpDir, "daemon-file.txt")
			Expect(os.WriteFile(filePath, []byte("daemon-copy-test"), 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--copy", fmt.Sprintf("%s:/tmp/daemon-file.txt", filePath),
				"--", "cat /tmp/daemon-file.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("daemon-copy-test"))
		})

		It("should support --clean flag to remove files before copy", func() {
			filePath := filepath.Join(tmpDir, "clean-test.txt")
			Expect(os.WriteFile(filePath, []byte("v1"), 0644)).To(Succeed())

			// First copy
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--copy", fmt.Sprintf("%s:/tmp/clean-dir/clean-test.txt", filePath),
				"--", "cat /tmp/clean-dir/clean-test.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("v1"))

			// Create a stale file in the target dir
			cmd = exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--", "sh -c 'echo stale > /tmp/clean-dir/stale.txt'")
			_, exitCode = RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0))

			// Update file content
			Expect(os.WriteFile(filePath, []byte("v2"), 0644)).To(Succeed())

			// Copy with --clean to remove stale files
			cmd = exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName,
				"-n", testNS,
				"--clean", "/tmp/clean-dir",
				"--copy", fmt.Sprintf("%s:/tmp/clean-dir/clean-test.txt", filePath),
				"--", "sh -c 'cat /tmp/clean-dir/clean-test.txt && ls /tmp/clean-dir/'")
			output, exitCode = RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("v2"))
			Expect(output).NotTo(ContainSubstring("stale.txt"), "stale file should be cleaned up")
		})
	})

	Context("copy with large file", func() {
		It("should copy a file larger than 1MB", func() {
			filePath := filepath.Join(tmpDir, "large.bin")
			data := make([]byte, 1024*1024+100) // ~1MB
			for i := range data {
				data[i] = byte(i % 256)
			}
			Expect(os.WriteFile(filePath, data, 0644)).To(Succeed())

			cmd := exec.Command("helm", "in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false",
				"--copy", fmt.Sprintf("%s:/tmp/large.bin", filePath),
				"--", "sh -c 'wc -c < /tmp/large.bin'")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(strings.TrimSpace(output)).To(ContainSubstring("1048676"))
		})
	})
})
