package hippod

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/noksa/go-helpers/helpers/gopointer"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/hipembedded"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// parseVolume parses a volume string in format: type:name:mountPath[:readOnly]
// Supported types:
//
//	pvc:claim-name:/mount/path[:ro]
//	secret:secret-name:/mount/path[:ro]
//	configmap:cm-name:/mount/path[:ro]
//	hostpath:host-path:/mount/path[:ro]
func parseVolume(s string) (corev1.Volume, corev1.VolumeMount, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 3 {
		return corev1.Volume{}, corev1.VolumeMount{}, fmt.Errorf("expected format type:name:mountPath[:ro], got %q", s)
	}

	volType := strings.ToLower(parts[0])
	name := parts[1]
	mountPath := parts[2]
	readOnly := len(parts) == 4 && parts[3] == "ro"

	// Sanitize volume name for k8s (must be DNS-compatible)
	volName := strings.ReplaceAll(name, "/", "-")
	volName = strings.ReplaceAll(volName, "_", "-")
	volName = strings.TrimLeft(volName, "-")
	if len(volName) > 63 {
		volName = volName[:63]
	}

	mount := corev1.VolumeMount{
		Name:      volName,
		MountPath: mountPath,
		ReadOnly:  readOnly,
	}

	var vol corev1.Volume
	switch volType {
	case "pvc":
		vol = corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: name,
					ReadOnly:  readOnly,
				},
			},
		}
	case "secret":
		vol = corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: name,
				},
			},
		}
	case "configmap":
		vol = corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: name},
				},
			},
		}
	case "hostpath":
		vol = corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: name,
				},
			},
		}
	default:
		return corev1.Volume{}, corev1.VolumeMount{}, fmt.Errorf("unsupported volume type %q, supported: pvc, secret, configmap, hostpath", volType)
	}

	return vol, mount, nil
}

func buildPodSpec(opts cmdoptions.ExecOptions, daemon bool) (corev1.PodSpec, error) {
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

	if !daemon && len(opts.CopyFrom) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  hipconsts.EnvWaitCopyDone,
			Value: "1",
		})
	}

	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}

	if opts.CpuRequest != "" && opts.CpuRequest != "0" {
		requests["cpu"] = resource.MustParse(opts.CpuRequest)
	}
	if opts.CpuLimit != "" && opts.CpuLimit != "0" {
		limits["cpu"] = resource.MustParse(opts.CpuLimit)
	}
	if opts.MemoryRequest != "" && opts.MemoryRequest != "0" {
		requests["memory"] = resource.MustParse(opts.MemoryRequest)
	}
	if opts.MemoryLimit != "" && opts.MemoryLimit != "0" {
		limits["memory"] = resource.MustParse(opts.MemoryLimit)
	}

	securityContext := &corev1.SecurityContext{}
	if opts.RunAsUser > -1 {
		securityContext.RunAsUser = gopointer.NewOf(opts.RunAsUser)
	}
	if opts.RunAsGroup > -1 {
		securityContext.RunAsGroup = gopointer.NewOf(opts.RunAsGroup)
	}

	// Parse volumes
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	for _, v := range opts.Volumes {
		vol, mount, err := parseVolume(v)
		if err != nil {
			return corev1.PodSpec{}, fmt.Errorf("invalid volume %q: %w", v, err)
		}
		volumes = append(volumes, vol)
		volumeMounts = append(volumeMounts, mount)
	}

	serviceAccountName := Namespace
	if opts.ServiceAccount != "" {
		serviceAccountName = opts.ServiceAccount
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
			VolumeMounts:    volumeMounts,
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
		Volumes:                       volumes,
		RestartPolicy:                 corev1.RestartPolicyNever,
		ServiceAccountName:            serviceAccountName,
		AutomountServiceAccountToken:  gopointer.NewOf(true),
		TerminationGracePeriodSeconds: gopointer.NewOf[int64](300),
	}
	if len(requests) > 0 || len(limits) > 0 {
		podSpec.Containers[0].Resources = corev1.ResourceRequirements{
			Requests: requests,
			Limits:   limits,
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

	if opts.ActiveDeadlineSeconds > 0 {
		podSpec.ActiveDeadlineSeconds = gopointer.NewOf(opts.ActiveDeadlineSeconds)
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

func buildDaemonPodSpec(opts cmdoptions.ExecOptions) (corev1.PodSpec, error) {
	podSpec, err := buildPodSpec(opts, true)
	if err != nil {
		return corev1.PodSpec{}, err
	}

	// Override command and args to run indefinitely with proper signal handling
	podSpec.Containers[0].Command = []string{"sh", "-c"}
	podSpec.Containers[0].Args = []string{"touch /tmp/ready && trap 'exit 0' TERM INT; sleep infinity & wait"}
	podSpec.Containers[0].Env = nil
	return podSpec, nil
}
