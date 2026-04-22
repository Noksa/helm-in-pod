//go:build e2e

package e2e

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

// generateReleaseName creates a unique release name for testing
func generateReleaseName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, randomString(6))
}

// generateNamespace creates a unique namespace name for testing
func generateNamespace(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, randomString(8))
}

// generateTestLabel creates a unique label for test isolation in parallel execution
func generateTestLabel() string {
	return fmt.Sprintf("test-id=%s", randomString(8))
}

// createNamespace creates a namespace and returns its name
func createNamespace(prefix string) string {
	ns := generateNamespace(prefix)
	cmd := exec.Command("kubectl", "create", "namespace", ns, "--dry-run=client", "-o", "yaml")
	output, _ := Run(cmd)
	cmd = exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(output)
	_, _ = Run(cmd)
	return ns
}

// deleteNamespace deletes a namespace
func deleteNamespace(ns string) {
	cmd := exec.Command("kubectl", "delete", "namespace", ns, "--ignore-not-found", "--wait=false")
	_, _ = Run(cmd)
}

// logOnFailure prints relevant logs when a test fails.
// Always call this BEFORE deleting namespaces or stopping daemons so pods still exist.
func logOnFailure(ns string) {
	if !CurrentSpecReport().Failed() {
		return
	}
	for _, namespace := range []string{ns, hipconsts.HelmInPodNamespace} {
		if namespace == "" {
			continue
		}
		GinkgoWriter.Printf("\n=== Pods in namespace %s ===\n", namespace)
		cmd := exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "wide")
		output, _ := Run(cmd)
		GinkgoWriter.Printf("%s\n", output)

		// describe all pods to get events and status
		cmd = exec.Command("kubectl", "describe", "pods", "-n", namespace)
		output, _ = Run(cmd)
		GinkgoWriter.Printf("\n=== Describe pods in %s ===\n%s\n", namespace, output)
	}
}

// createTestChart creates a minimal test chart in a temporary directory
func createTestChart(name string) string {
	chartDir := fmt.Sprintf("/tmp/helm-chart-%s-%s", name, randomString(6))

	// Create chart structure
	cmd := exec.Command("mkdir", "-p", fmt.Sprintf("%s/templates", chartDir))
	_, _ = Run(cmd)

	// Chart.yaml
	chartYaml := fmt.Sprintf(`apiVersion: v2
name: %s
description: Test chart for e2e
type: application
version: 0.1.0
appVersion: "1.0"
`, name)
	cmd = exec.Command("sh", "-c", fmt.Sprintf("cat > %s/Chart.yaml << 'EOF'\n%s\nEOF", chartDir, chartYaml))
	_, _ = Run(cmd)

	// values.yaml
	valuesYaml := `replicaCount: 1
image:
  repository: nginx
  tag: "1.21"
`
	cmd = exec.Command("sh", "-c", fmt.Sprintf("cat > %s/values.yaml << 'EOF'\n%s\nEOF", chartDir, valuesYaml))
	_, _ = Run(cmd)

	// deployment.yaml
	deploymentYaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
    spec:
      containers:
      - name: nginx
        image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
        ports:
        - containerPort: 80
`
	cmd = exec.Command("sh", "-c", fmt.Sprintf("cat > %s/templates/deployment.yaml << 'EOF'\n%s\nEOF", chartDir, deploymentYaml))
	_, _ = Run(cmd)

	return chartDir
}

// cleanupChart removes the temporary chart directory
func cleanupChart(chartDir string) {
	cmd := exec.Command("rm", "-rf", chartDir)
	_, _ = Run(cmd)
}
