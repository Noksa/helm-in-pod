package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/spf13/cobra"
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
	cmd.Flags().Int64Var(&opts.RunAsUser, "run-as-user", -1, "Run as user ID to be set in security context. Omitted if not specified and default from an image is used")
	cmd.Flags().Int64Var(&opts.RunAsGroup, "run-as-group", -1, "Run as group ID to be set in security context. Omitted if not specified and default from an image is used")
	cmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels to be set on a pod")
	cmd.Flags().StringToStringVar(&opts.Annotations, "annotations", map[string]string{}, "Additional annotations to be set on a pod")
	cmd.Flags().BoolVar(&opts.CreatePDB, "create-pdb", true, "Create PodDisruptionBudget to protect the pod from voluntary disruptions during operations")

	// Deprecated flags
	cmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit (deprecated: use --cpu-request and --cpu-limit)")
	cmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit (deprecated: use --memory-request and --memory-limit)")
	_ = cmd.Flags().MarkDeprecated("cpu", "use --cpu-request and --cpu-limit instead")
	_ = cmd.Flags().MarkDeprecated("memory", "use --memory-request and --memory-limit instead")

	// New flags for separate requests and limits
	cmd.Flags().StringVar(&opts.CpuRequest, "cpu-request", "", "Pod's cpu request")
	cmd.Flags().StringVar(&opts.CpuLimit, "cpu-limit", "", "Pod's cpu limit (optional)")
	cmd.Flags().StringVar(&opts.MemoryRequest, "memory-request", "", "Pod's memory request")
	cmd.Flags().StringVar(&opts.MemoryLimit, "memory-limit", "", "Pod's memory limit (optional)")

	cmd.Flags().BoolVar(&opts.HostNetwork, "host-network", false, "Use host network in a pod")
	cmd.Flags().StringSliceVar(&opts.Tolerations, "tolerations", []string{}, "Pod's tolerations in format key=value:effect:operator. Examples: '::Exists' (all taints), 'key=::Exists' (key with any effect), 'key=:NoSchedule:Exists', 'key=value:NoSchedule:Equal'")
	cmd.Flags().StringToStringVar(&opts.NodeSelector, "node-selector", map[string]string{}, "Pod's node selectors. Examples: 'node-role.kubernetes.io/control-plane=\"\"', 'disktype=ssd'")
	cmd.Flags().StringVar(&opts.ImagePullSecret, "image-pull-secret", "", "Specify an image pull secret which should be used to pull --image from private repository")
	cmd.Flags().StringVar(&opts.PullPolicy, "pull-policy", "IfNotPresent", "Image pull policy to use in helm pod")
	cmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.34.5-v4.1.1", "An image which will be used")
	cmd.Flags().StringSliceVar(&opts.Volumes, "volume", []string{}, "Mount volumes in the pod. Format: type:name:mountPath[:ro]. Types: pvc, secret, configmap, hostpath. Examples: 'pvc:my-claim:/data', 'secret:my-secret:/etc/creds:ro', 'configmap:my-cm:/etc/config', 'hostpath:/var/log:/host-logs:ro'")
	cmd.Flags().StringVar(&opts.ServiceAccount, "service-account", "", "Service account to use in the pod (default: helm-in-pod)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Print the pod spec as YAML without creating the pod")
	cmd.Flags().Int64Var(&opts.ActiveDeadlineSeconds, "active-deadline-seconds", 0, "Maximum duration in seconds the pod is allowed to run. The pod will be terminated by Kubernetes once this deadline is exceeded, regardless of whether the client is still connected. Useful to avoid orphaned pods in CI/CD pipelines. 0 means no deadline (default)")
}

func addRuntimeFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions, copyRepoDefault bool) {
	cmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set in helm's pod before running a command")
	cmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Environment variables to be substituted in helm's pod (WITHOUT values). Values will be substituted from exported env on host")
	cmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", copyRepoDefault, "Copy existing helm repositories to helm pod")
	cmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "A list of helm repository aliases which should be updated before running a command. Applicable only if --copy-repo set to true. All repositories will be updated if not specified")
	cmd.Flags().StringSliceVarP(&opts.Files, "copy", "c", []string{}, "A map of files/directories which should be copied from host to container. Can be specified multiple times. Example: -c /path_on_host/values.yaml:/path_in_container/values.yaml")
	cmd.Flags().IntVar(&opts.CopyAttempts, "copy-attempts", 3, "Attempts count in each copy action (in copy-repo and copy flags). If your connection to k8s api is no stable, you can try to increase the attempts")
	cmd.Flags().IntVar(&opts.UpdateRepoAttempts, "update-repo-attempts", 3, "Attempts count in each helm update repo action. Applicable only if copy-repo set to true")
	cmd.Flags().StringSliceVar(&opts.CopyFrom, "copy-from", []string{}, "Copy files/directories from pod to host after command execution. Format: /path_in_pod:/path_on_host. Example: --copy-from /tmp/output.yaml:./output.yaml")
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
