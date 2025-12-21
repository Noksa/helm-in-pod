package helpers

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/Noksa/operator-home/pkg/operatorkclient"
	"github.com/noksa/helm-in-pod/internal"
)

// GetHelmMajorVersion returns the major version of Helm running in the specified pod
// Returns 0 if version cannot be determined
func GetHelmMajorVersion(podName, podNamespace string) (int, error) {
	stdout, stderr, err := operatorkclient.RunCommandInPod("helm version --template '{{ $.Version }}'", internal.HelmInPodNamespace, podName, podNamespace, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get helm version: %v, stderr: %s", err, stderr)
	}

	// Extract major version from output like "v3.14.0" or "v4.0.0"
	re := regexp.MustCompile(`v(\d+)\.`)
	matches := re.FindStringSubmatch(stdout)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse helm version from output: %s", stdout)
	}

	majorVersion, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid major version: %s", matches[1])
	}

	return majorVersion, nil
}

// IsHelm4 checks if the Helm version in pod equals 4
func IsHelm4(podName, podNamespace string) (bool, error) {
	major, err := GetHelmMajorVersion(podName, podNamespace)
	if err != nil {
		return false, err
	}
	return major == 4, nil
}
