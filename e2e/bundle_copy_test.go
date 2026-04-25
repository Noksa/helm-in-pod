//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This test suite exercises the batch bundle copy optimization introduced in
// feat/batch-copy-bundle. It replicates the real-world usage pattern:
//
//   helm in-pod exec \
//     --copy-files ./chart:/tmp/chart \
//     --copy-files ./values-env.yaml:/tmp/values-env.yaml \
//     -- helm template myapp /tmp/chart -f /tmp/values-env.yaml
//
// That is: a full Helm chart directory (Chart.yaml + templates/) plus one or more
// per-environment override YAML files, all sent to the pod in a single ExecInPod
// together with the wrapped script and boot-info collection.
var _ = Describe("Bundle Copy (chart dir + values files)", func() {
	var (
		testNS    string
		testLabel string
		chartDir  string
		tmpDir    string
	)

	BeforeEach(func() {
		testNS = createNamespace("e2e-bundle")
		testLabel = generateTestLabel()
		DeferCleanup(func() { deleteNamespace(testNS) })

		var err error
		tmpDir, err = os.MkdirTemp("", "hip-bundle-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Build a realistic chart layout:
		//   <chartDir>/
		//     Chart.yaml
		//     values.yaml          ← default values
		//     templates/
		//       deployment.yaml
		//       service.yaml
		chartDir = filepath.Join(tmpDir, "mychart")
		Expect(os.MkdirAll(filepath.Join(chartDir, "templates"), 0755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(
			"apiVersion: v2\nname: mychart\ndescription: Bundle test chart\ntype: application\nversion: 0.1.0\nappVersion: \"1.0\"\n",
		), 0644)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(
			"replicaCount: 1\nimage:\n  repository: nginx\n  tag: \"1.21\"\nenv: default\n",
		), 0644)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(chartDir, "templates", "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  labels:
    env: {{ .Values.env }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
        env: {{ .Values.env }}
    spec:
      containers:
      - name: app
        image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
`), 0644)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(chartDir, "templates", "service.yaml"), []byte(`apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}
spec:
  selector:
    app: {{ .Release.Name }}
  ports:
  - port: 80
    targetPort: 80
`), 0644)).To(Succeed())
	})

	AfterEach(func() {
		logOnFailure(testNS)
		_ = os.RemoveAll(tmpDir)
	})

	Context("chart directory + one override values file", func() {
		It("should copy chart dir and values file and run helm template", func() {
			// Per-environment override (the pattern the user described)
			envValues := filepath.Join(tmpDir, "values-qa.yaml")
			Expect(os.WriteFile(envValues, []byte("replicaCount: 3\nenv: qa\n"), 0644)).To(Succeed())

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args,
				"--copy", fmt.Sprintf("%s:/tmp/mychart", chartDir),
				"--copy", fmt.Sprintf("%s:/tmp/values-qa.yaml", envValues),
				"--", "helm template myapp /tmp/mychart -f /tmp/values-qa.yaml",
			)
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "helm template failed:\n%s", output)
			Expect(output).To(ContainSubstring("env: qa"), "env override not applied")
			Expect(output).To(ContainSubstring("replicas: 3"), "replicaCount override not applied")
		})
	})

	Context("chart directory + multiple override values files", func() {
		It("should copy chart dir and multiple values files in a single bundle", func() {
			// Simulate multiple env layers (base override + secret overlay)
			valuesBase := filepath.Join(tmpDir, "values-base.yaml")
			valuesSecret := filepath.Join(tmpDir, "values-secret.yaml")
			Expect(os.WriteFile(valuesBase, []byte("replicaCount: 5\nenv: staging\n"), 0644)).To(Succeed())
			Expect(os.WriteFile(valuesSecret, []byte("image:\n  tag: \"1.25\"\n"), 0644)).To(Succeed())

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args,
				"--copy", fmt.Sprintf("%s:/tmp/mychart", chartDir),
				"--copy", fmt.Sprintf("%s:/tmp/values-base.yaml", valuesBase),
				"--copy", fmt.Sprintf("%s:/tmp/values-secret.yaml", valuesSecret),
				"--", "helm template myapp /tmp/mychart -f /tmp/values-base.yaml -f /tmp/values-secret.yaml",
			)
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "helm template failed:\n%s", output)
			Expect(output).To(ContainSubstring("env: staging"))
			Expect(output).To(ContainSubstring("replicas: 5"))
			Expect(output).To(ContainSubstring("1.25"))
		})
	})

	Context("chart directory + values files + verify all template files are intact", func() {
		It("should preserve all chart template files after bundle extraction", func() {
			envValues := filepath.Join(tmpDir, "values-prod.yaml")
			Expect(os.WriteFile(envValues, []byte("env: prod\n"), 0644)).To(Succeed())

			// Verify both templates are present and helm template renders both
			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args,
				"--copy", fmt.Sprintf("%s:/tmp/mychart", chartDir),
				"--copy", fmt.Sprintf("%s:/tmp/values-prod.yaml", envValues),
				"--", "helm template myapp /tmp/mychart -f /tmp/values-prod.yaml",
			)
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "helm template failed:\n%s", output)
			// Both deployment and service should appear
			Expect(output).To(ContainSubstring("kind: Deployment"))
			Expect(output).To(ContainSubstring("kind: Service"))
			Expect(output).To(ContainSubstring("env: prod"))
		})
	})

	Context("parallel bundle copies (simulating concurrent deployments)", func() {
		It("should handle multiple concurrent helm in-pod execs copying the same chart", func() {
			envValues := filepath.Join(tmpDir, "values-parallel.yaml")
			Expect(os.WriteFile(envValues, []byte("env: parallel\n"), 0644)).To(Succeed())

			// Fire 5 concurrent execs, each copying the full chart + values and running
			// helm template. This validates correctness under parallelism.
			const concurrency = 5
			type result struct {
				output   string
				exitCode int
			}
			results := make(chan result, concurrency)
			for i := 0; i < concurrency; i++ {
				go func(idx int) {
					label := fmt.Sprintf("test-id=parallel-%s-%d", randomString(4), idx)
					args := []string{"in-pod", "exec",
						"--labels", label,
						"--copy-repo=false"}
					args = append(args, e2eResourceFlags...)
					args = append(args,
						"--copy", fmt.Sprintf("%s:/tmp/mychart", chartDir),
						"--copy", fmt.Sprintf("%s:/tmp/values-parallel.yaml", envValues),
						"--", "helm template myapp /tmp/mychart -f /tmp/values-parallel.yaml",
					)
					cmd := exec.Command("helm", args...)
					out, code := RunWithExitCode(cmd)
					results <- result{output: out, exitCode: code}
				}(i)
			}

			// Collect all results within 3 minutes
			timeout := time.After(3 * time.Minute)
			passed := 0
			for i := 0; i < concurrency; i++ {
				select {
				case r := <-results:
					Expect(r.exitCode).To(Equal(0), "parallel exec %d failed:\n%s", i, r.output)
					Expect(r.output).To(ContainSubstring("env: parallel"), "parallel exec %d wrong output", i)
					passed++
				case <-timeout:
					Fail(fmt.Sprintf("Timeout waiting for parallel results after %d/%d completed", passed, concurrency))
				}
			}
			Expect(passed).To(Equal(concurrency))
		})
	})

	Context("large chart with many template files", func() {
		It("should handle chart directory with many template files correctly", func() {
			// Create a chart with 20 template files (simulates real charts with many resources)
			for i := 1; i <= 20; i++ {
				content := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cm-%d
data:
  key: "value-%d"
`, i, i)
				fname := fmt.Sprintf("templates/cm-%02d.yaml", i)
				Expect(os.WriteFile(filepath.Join(chartDir, fname), []byte(content), 0644)).To(Succeed())
			}

			envValues := filepath.Join(tmpDir, "values-large.yaml")
			Expect(os.WriteFile(envValues, []byte("env: large\n"), 0644)).To(Succeed())

			args := []string{"in-pod", "exec",
				"--labels", testLabel,
				"--copy-repo=false"}
			args = append(args, e2eResourceFlags...)
			args = append(args,
				"--copy", fmt.Sprintf("%s:/tmp/mychart", chartDir),
				"--copy", fmt.Sprintf("%s:/tmp/values-large.yaml", envValues),
				"--", "helm template myapp /tmp/mychart -f /tmp/values-large.yaml",
			)
			cmd := exec.Command("helm", args...)
			output, exitCode := RunWithExitCode(cmd)
			Expect(exitCode).To(Equal(0), "helm template failed:\n%s", output)

			// Verify all 20 configmaps were rendered
			for i := 1; i <= 20; i++ {
				Expect(output).To(ContainSubstring(fmt.Sprintf("value-%d", i)),
					"ConfigMap %d missing from output", i)
			}
			Expect(strings.Count(output, "kind: ConfigMap")).To(Equal(20))
		})
	})
})
