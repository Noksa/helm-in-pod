package hippod

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/noksa/go-helpers/helpers/gopointer"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipembedded"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func buildPodSpec(opts cmdoptions.ExecOptions) (corev1.PodSpec, error) {
	var envVars []corev1.EnvVar
	for _, env := range opts.SubstEnv {
		val := os.Getenv(env)
		envVars = append(envVars, corev1.EnvVar{
			Name:  env,
			Value: val,
		})
	}
	for k, v := range opts.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "TIMEOUT",
		Value: strconv.Itoa(int(opts.Timeout.Seconds())),
	})

	resourceList := corev1.ResourceList{}

	if opts.Cpu != "" && opts.Cpu != "0" {
		resourceList["cpu"] = resource.MustParse(opts.Cpu)
	}
	if opts.Memory != "" && opts.Memory != "0" {
		resourceList["memory"] = resource.MustParse(opts.Memory)
	}
	securityContext := &corev1.SecurityContext{}
	if opts.RunAsUser > -1 {
		securityContext.RunAsUser = gopointer.NewOf(opts.RunAsUser)
	}
	if opts.RunAsGroup > -1 {
		securityContext.RunAsGroup = gopointer.NewOf(opts.RunAsGroup)
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            Namespace,
			ImagePullPolicy: corev1.PullPolicy(opts.PullPolicy),
			Image:           opts.Image,
			Command:         []string{"sh", "-cue"},
			Env:             envVars,
			SecurityContext: securityContext,
			Args:            []string{hipembedded.GetShScript()},
			WorkingDir:      "/",
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "([ -f /tmp/ready ] && exit 0) || exit 1"},
					},
				},
				TimeoutSeconds:   2,
				PeriodSeconds:    1,
				SuccessThreshold: 1,
				FailureThreshold: 60,
			},
		}},
		RestartPolicy:                 corev1.RestartPolicyNever,
		ServiceAccountName:            Namespace,
		AutomountServiceAccountToken:  gopointer.NewOf(true),
		TerminationGracePeriodSeconds: gopointer.NewOf[int64](300),
	}
	if len(resourceList) > 0 {
		podSpec.Resources = &corev1.ResourceRequirements{
			Requests: resourceList,
			Limits:   resourceList,
		}
	}

	if opts.ImagePullSecret != "" {
		podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets,
			corev1.LocalObjectReference{Name: opts.ImagePullSecret})
	}

	seen := make(map[string]bool)
	for _, t := range opts.Tolerations {
		if seen[t] {
			return corev1.PodSpec{}, fmt.Errorf("duplicate toleration %q", t)
		}
		seen[t] = true
		toleration, err := parseToleration(t)
		if err != nil {
			return corev1.PodSpec{}, fmt.Errorf("invalid toleration %q: %w", t, err)
		}
		podSpec.Tolerations = append(podSpec.Tolerations, toleration)
	}

	if len(opts.NodeSelector) > 0 {
		podSpec.NodeSelector = opts.NodeSelector
	}

	if opts.HostNetwork {
		podSpec.HostNetwork = true
	}

	return podSpec, nil
}

// parseToleration parses a toleration string in format: key=value:effect:operator
// Examples:
//
//	"::Exists" - tolerate all taints
//	"key=::Exists" - tolerate key with any effect
//	"key=:effect:Exists" - tolerate specific key with any value
//	"key=value:effect:Equal" - tolerate specific key=value
func parseToleration(s string) (corev1.Toleration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return corev1.Toleration{}, fmt.Errorf("expected format key=value:effect:operator, got %q", s)
	}

	operator := corev1.TolerationOperator(parts[2])
	if operator != corev1.TolerationOpEqual && operator != corev1.TolerationOpExists {
		return corev1.Toleration{}, fmt.Errorf("operator must be Equal or Exists, got %q", parts[2])
	}

	toleration := corev1.Toleration{
		Operator: operator,
	}

	// Effect can be empty to match all effects
	if parts[1] != "" {
		toleration.Effect = corev1.TaintEffect(parts[1])
	}

	// Key and value parsing
	if parts[0] != "" {
		keyValue := strings.SplitN(parts[0], "=", 2)
		toleration.Key = keyValue[0]
		if len(keyValue) == 2 {
			toleration.Value = keyValue[1]
		}
	}

	return toleration, nil
}
