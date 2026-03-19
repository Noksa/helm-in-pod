//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Mount", func() {
	var (
		testNS    string
		testLabel string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-volume")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })
	})

	AfterEach(func() {
		logOnFailure(testNS)
	})

	Context("pvc volume", func() {
		var pvcName string

		BeforeEach(func() {
			pvcName = fmt.Sprintf("test-pvc-%s", randomString(6))
			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(fmt.Sprintf(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  namespace: helm-in-pod
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Mi`, pvcName))
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create PVC: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "pvc", pvcName, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
		})

		It("should mount a PVC volume in exec mode", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--volume", fmt.Sprintf("pvc:%s:/data", pvcName),
				"--",
				"sh -c 'echo pvc-test > /data/test.txt && cat /data/test.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("pvc-test"))
		})
	})

	Context("hostpath volume", func() {
		var hostDir string

		BeforeEach(func() {
			hostDir = fmt.Sprintf("/tmp/e2e-hostpath-%s", randomString(6))
			cluster := "helm-in-pod-e2e"
			if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
				cluster = v
			}
			nodeName := fmt.Sprintf("%s-control-plane", cluster)

			cmd := exec.Command("docker", "exec", nodeName, "mkdir", "-p", hostDir)
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create host dir: %s", output)

			cmd = exec.Command("docker", "exec", nodeName, "sh", "-c",
				fmt.Sprintf("echo hostpath-content > %s/test.txt", hostDir))
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to write host file: %s", output)

			DeferCleanup(func() {
				cmd := exec.Command("docker", "exec", nodeName, "rm", "-rf", hostDir)
				_, _ = Run(cmd)
			})
		})

		It("should mount a hostpath volume and read host files", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--volume", fmt.Sprintf("hostpath:%s:/host-data:ro", hostDir),
				"--",
				"cat /host-data/test.txt",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("hostpath-content"))
		})
	})

	Context("configmap volume", func() {
		var cmName string

		BeforeEach(func() {
			cmName = fmt.Sprintf("test-cm-%s", randomString(6))
			cmd := exec.Command("kubectl", "create", "configmap", cmName,
				"--from-literal=app.conf=key=value",
				"-n", "helm-in-pod")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create configmap: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "configmap", cmName, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
		})

		It("should mount a configmap volume in exec mode", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--volume", fmt.Sprintf("configmap:%s:/etc/config", cmName),
				"--",
				"cat /etc/config/app.conf",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("key=value"))
		})
	})

	Context("secret volume", func() {
		var secretName string

		BeforeEach(func() {
			secretName = fmt.Sprintf("test-secret-%s", randomString(6))
			cmd := exec.Command("kubectl", "create", "secret", "generic", secretName,
				"--from-literal=password=s3cret",
				"-n", "helm-in-pod")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create secret: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "secret", secretName, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
		})

		It("should mount a secret volume read-only in exec mode", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--volume", fmt.Sprintf("secret:%s:/etc/creds:ro", secretName),
				"--",
				"cat /etc/creds/password",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("s3cret"))
		})
	})

	Context("volume in daemon mode", func() {
		var (
			daemonName string
			cmName     string
		)

		BeforeEach(func() {
			cmName = fmt.Sprintf("daemon-cm-%s", randomString(6))
			cmd := exec.Command("kubectl", "create", "configmap", cmName,
				"--from-literal=data.txt=daemon-volume-test",
				"-n", "helm-in-pod")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create configmap: %s", output)

			daemonName = fmt.Sprintf("vol-daemon-%s", randomString(6))
			cmd = BuildDaemonStartCommand(
				"--name", daemonName,
				"--labels", testLabel,
				"--volume", fmt.Sprintf("configmap:%s:/tmp/work", cmName),
				"-n", testNS,
			)
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to start daemon: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "stop", "--name", daemonName, "-n", testNS)
			_, _ = Run(cmd)
			cmd = exec.Command("kubectl", "delete", "configmap", cmName, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
		})

		It("should mount configmap volume in daemon pod", func() {
			cmd := exec.Command("helm", "in-pod", "daemon", "exec",
				"--name", daemonName, "-n", testNS,
				"--", "cat /tmp/work/data.txt")
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("daemon-volume-test"))
		})
	})

	Context("multiple volumes", func() {
		var (
			cm1Name string
			cm2Name string
		)

		BeforeEach(func() {
			cm1Name = fmt.Sprintf("multi-cm1-%s", randomString(6))
			cm2Name = fmt.Sprintf("multi-cm2-%s", randomString(6))
			cmd := exec.Command("kubectl", "create", "configmap", cm1Name,
				"--from-literal=a.txt=alpha",
				"-n", "helm-in-pod")
			output, err := Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create configmap: %s", output)

			cmd = exec.Command("kubectl", "create", "configmap", cm2Name,
				"--from-literal=b.txt=bravo",
				"-n", "helm-in-pod")
			output, err = Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create configmap: %s", output)
		})

		AfterEach(func() {
			cmd := exec.Command("kubectl", "delete", "configmap", cm1Name, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
			cmd = exec.Command("kubectl", "delete", "configmap", cm2Name, "-n", "helm-in-pod", "--ignore-not-found")
			_, _ = Run(cmd)
		})

		It("should mount multiple volumes simultaneously", func() {
			cmd := BuildHelmInPodCommand(
				"--labels", testLabel,
				"--volume", fmt.Sprintf("configmap:%s:/mnt/vol1", cm1Name),
				"--volume", fmt.Sprintf("configmap:%s:/mnt/vol2", cm2Name),
				"--",
				"sh -c 'cat /mnt/vol1/a.txt && cat /mnt/vol2/b.txt'",
			)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "output: %s", output)
			Expect(output).To(ContainSubstring("alpha"))
			Expect(output).To(ContainSubstring("bravo"))
		})
	})
})
