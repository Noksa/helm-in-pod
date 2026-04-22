package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
)

func getDaemonName(name string) (string, error) {
	if name == "" {
		name = os.Getenv(hipconsts.EnvDaemonName)
	}
	if name == "" {
		return "", fmt.Errorf("--name is required (or set %s)", hipconsts.EnvDaemonName)
	}
	return name, nil
}

func addExecOptionsFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions) {
	addPodCreationFlags(cmd, opts)
	addRuntimeFlags(cmd, opts, true)
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		return validateResourceFlags(cmd, opts)
	}
}

func validateResourceFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions) error {
	cpuChanged := cmd.Flags().Changed("cpu")
	memoryChanged := cmd.Flags().Changed("memory")
	cpuRequestChanged := cmd.Flags().Changed("cpu-request")
	cpuLimitChanged := cmd.Flags().Changed("cpu-limit")
	memoryRequestChanged := cmd.Flags().Changed("memory-request")
	memoryLimitChanged := cmd.Flags().Changed("memory-limit")

	// Check if old and new CPU flags are used together
	if cpuChanged && (cpuRequestChanged || cpuLimitChanged) {
		return fmt.Errorf("cannot use --cpu with --cpu-request or --cpu-limit")
	}

	// Check if old and new memory flags are used together
	if memoryChanged && (memoryRequestChanged || memoryLimitChanged) {
		return fmt.Errorf("cannot use --memory with --memory-request or --memory-limit")
	}

	// If new flags are not set, use defaults from old flags
	if !cpuRequestChanged && !cpuLimitChanged && !cpuChanged {
		// No flags set, use default
		opts.CpuRequest = "1100m"
		opts.CpuLimit = "1100m"
	} else if cpuChanged {
		// Old flag used, set both request and limit to same value
		opts.CpuRequest = opts.Cpu
		opts.CpuLimit = opts.Cpu
	}

	if !memoryRequestChanged && !memoryLimitChanged && !memoryChanged {
		// No flags set, use default
		opts.MemoryRequest = "500Mi"
		opts.MemoryLimit = "500Mi"
	} else if memoryChanged {
		// Old flag used, set both request and limit to same value
		opts.MemoryRequest = opts.Memory
		opts.MemoryLimit = opts.Memory
	}

	return nil
}

func addPodCreationFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions) {
	cmd.Flags().Int64Var(&opts.RunAsUser, "run-as-user", -1, "Security context user ID. Omitted if not set, using the image default")
	cmd.Flags().Int64Var(&opts.RunAsGroup, "run-as-group", -1, "Security context group ID. Omitted if not set, using the image default")
	cmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels for the pod")
	cmd.Flags().StringToStringVar(&opts.Annotations, "annotations", map[string]string{}, "Additional annotations for the pod")
	cmd.Flags().BoolVar(&opts.CreatePDB, "create-pdb", true, "Create PodDisruptionBudget to protect the pod from voluntary disruptions during operations")

	// Deprecated flags
	cmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit (deprecated: use --cpu-request and --cpu-limit)")
	cmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit (deprecated: use --memory-request and --memory-limit)")
	_ = cmd.Flags().MarkDeprecated("cpu", "use --cpu-request and --cpu-limit instead")
	_ = cmd.Flags().MarkDeprecated("memory", "use --memory-request and --memory-limit instead")

	// New flags for separate requests and limits
	cmd.Flags().StringVar(&opts.CpuRequest, "cpu-request", "", "Pod's CPU request (default: 1100m)")
	cmd.Flags().StringVar(&opts.CpuLimit, "cpu-limit", "", "Pod's CPU limit (default: 1100m)")
	cmd.Flags().StringVar(&opts.MemoryRequest, "memory-request", "", "Pod's memory request (default: 500Mi)")
	cmd.Flags().StringVar(&opts.MemoryLimit, "memory-limit", "", "Pod's memory limit (default: 500Mi)")

	cmd.Flags().BoolVar(&opts.HostNetwork, "host-network", false, "Use host network in the pod")
	cmd.Flags().StringSliceVar(&opts.Tolerations, "tolerations", []string{}, "Pod tolerations in format key=value:effect:operator. Examples: '::Exists' (all taints), 'key=::Exists' (key with any effect), 'key=:NoSchedule:Exists', 'key=value:NoSchedule:Equal'")
	cmd.Flags().StringToStringVar(&opts.NodeSelector, "node-selector", map[string]string{}, "Pod node selectors. Examples: 'node-role.kubernetes.io/control-plane=\"\"', 'disktype=ssd'")
	cmd.Flags().StringVar(&opts.ImagePullSecret, "image-pull-secret", "", "Image pull secret for pulling from a private registry")
	cmd.Flags().StringVar(&opts.PullPolicy, "pull-policy", "IfNotPresent", "Image pull policy for the pod")
	cmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1", "Docker image to use for the pod")
	cmd.Flags().StringSliceVar(&opts.Volumes, "volume", []string{}, "Mount volumes in the pod. Format: type:name:mountPath[:ro]. Types: pvc, secret, configmap, hostpath. Examples: 'pvc:my-claim:/data', 'secret:my-secret:/etc/creds:ro', 'configmap:my-cm:/etc/config', 'hostpath:/var/log:/host-logs:ro'")
	cmd.Flags().StringVar(&opts.ServiceAccount, "service-account", "", "Service account to use in the pod (default: helm-in-pod)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the pod spec as YAML without creating the pod")
	cmd.Flags().Int64Var(&opts.ActiveDeadlineSeconds, "active-deadline-seconds", 0, "Maximum duration in seconds the pod is allowed to run. The pod will be terminated by Kubernetes once this deadline is exceeded, regardless of whether the client is still connected. Useful to avoid orphaned pods in CI/CD pipelines. 0 means no deadline (default)")
}

func addRuntimeFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions, copyRepoDefault bool) {
	cmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to set in the pod before running the command")
	cmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Forward environment variables from the host to the pod by name (values are resolved from the host). Example: -s HELM_DRIVER,HELM_DRIVER_SQL_CONNECTION_STRING")
	cmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", copyRepoDefault, "Copy Helm repositories from the host to the pod")
	cmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "Helm repository aliases to update in the pod after copying. Requires --copy-repo. If specified without values, all repositories are updated")
	cmd.Flags().StringSliceVarP(&opts.Files, "copy", "c", []string{}, "Copy files/directories from host to pod. Format: /host/path:/pod/path. Repeatable")
	cmd.Flags().IntVar(&opts.CopyAttempts, "copy-attempts", 3, "Retry count for file copy operations (default: 3)")
	cmd.Flags().IntVar(&opts.UpdateRepoAttempts, "update-repo-attempts", 3, "Retry count for Helm repo update operations (default: 3)")
	cmd.Flags().StringSliceVar(&opts.CopyFrom, "copy-from", []string{}, "Copy files/directories from pod to host after execution. Format: /pod/path:/host/path. Repeatable")
}

// parseCopyFromMappings parses --copy-from flag values into a map of pod_path -> host_path.
func parseCopyFromMappings(copyFrom []string) (map[string]string, error) {
	result := map[string]string{}
	for _, val := range copyFrom {
		parts := strings.SplitN(val, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid --copy-from format %q, expected /pod/path:/host/path", val)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}
