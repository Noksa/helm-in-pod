//go:build e2e

package e2e

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"

	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

// reportsDir is where diagnostic files are written on test failure.
// Override with E2E_REPORTS_DIR env var; defaults to "e2e-reports".
func reportsDir() string {
	if d := os.Getenv("E2E_REPORTS_DIR"); d != "" {
		return d
	}
	return "e2e-reports"
}

// safeFilename converts a test name into a safe file system name.
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func safeFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// runDiagnostic runs a command purely for diagnostic output — no GinkgoWriter printing.
func runDiagnostic(cmd *exec.Cmd) string {
	dir, _ := GetProjectDir()
	cmd.Dir = dir
	cmd.Env = os.Environ()
	output, _ := cmd.CombinedOutput()
	return string(output)
}

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

// logOnFailure writes diagnostic information to a file when a test fails.
// Call this BEFORE deleting namespaces or stopping daemons so pods still exist.
// A short pointer to the file is printed to GinkgoWriter; the full output
// (kubectl get/describe) stays out of the console to keep CI logs readable.
func logOnFailure(ns string) {
	if !CurrentSpecReport().Failed() {
		return
	}

	dir := reportsDir()
	_ = os.MkdirAll(dir, 0o755)
	name := safeFilename(CurrentSpecReport().FullText())
	reportPath := filepath.Join(dir, name+".txt")

	var buf strings.Builder
	for _, namespace := range []string{ns, hipconsts.Namespace} {
		if namespace == "" {
			continue
		}
		fmt.Fprintf(&buf, "\n=== kubectl get pods -n %s ===\n", namespace)
		buf.WriteString(runDiagnostic(exec.Command("kubectl", "get", "pods", "-n", namespace, "-o", "wide")))
		fmt.Fprintf(&buf, "\n=== kubectl describe pods -n %s ===\n", namespace)
		buf.WriteString(runDiagnostic(exec.Command("kubectl", "describe", "pods", "-n", namespace)))
		fmt.Fprintf(&buf, "\n=== kubectl get events -n %s ===\n", namespace)
		buf.WriteString(runDiagnostic(exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")))
	}

	_ = os.WriteFile(reportPath, []byte(buf.String()), 0o644)
	GinkgoWriter.Printf("\n[DIAGNOSTICS] → %s\n", reportPath)
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
