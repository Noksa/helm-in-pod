package cmd

import (
	"fmt"
	"os"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/spf13/cobra"
)

func getDaemonName(name string) (string, error) {
	if name == "" {
		name = os.Getenv(internal.EnvDaemonName)
	}
	if name == "" {
		return "", fmt.Errorf("--name is required (or set %s)", internal.EnvDaemonName)
	}
	return name, nil
}

func addExecOptionsFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions) {
	addPodCreationFlags(cmd, opts)
	addRuntimeFlags(cmd, opts, true)
}

func addPodCreationFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions) {
	cmd.Flags().Int64Var(&opts.RunAsUser, "run-as-user", -1, "Run as user ID to be set in security context. Omitted if not specified and default from an image is used")
	cmd.Flags().Int64Var(&opts.RunAsGroup, "run-as-group", -1, "Run as group ID to be set in security context. Omitted if not specified and default from an image is used")
	cmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels to be set on a pod")
	cmd.Flags().StringToStringVar(&opts.Annotations, "annotations", map[string]string{}, "Additional annotations to be set on a pod")
	cmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit")
	cmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit")
	cmd.Flags().BoolVar(&opts.HostNetwork, "host-network", false, "Use host network in a pod")
	cmd.Flags().StringSliceVar(&opts.Tolerations, "tolerations", []string{}, "Pod's tolerations in format key=value:effect:operator. Examples: '::Exists' (all taints), 'key=::Exists' (key with any effect), 'key=:NoSchedule:Exists', 'key=value:NoSchedule:Equal'")
	cmd.Flags().StringToStringVar(&opts.NodeSelector, "node-selector", map[string]string{}, "Pod's node selectors. Examples: 'node-role.kubernetes.io/control-plane=\"\"', 'disktype=ssd'")
	cmd.Flags().StringVar(&opts.ImagePullSecret, "image-pull-secret", "", "Specify an image pull secret which should be used to pull --image from private repository")
	cmd.Flags().StringVar(&opts.PullPolicy, "pull-policy", "IfNotPresent", "Image pull policy to use in helm pod")
	cmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.34.2-v4.0.4", "An image which will be used")
}

func addRuntimeFlags(cmd *cobra.Command, opts *cmdoptions.ExecOptions, copyRepoDefault bool) {
	cmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set in helm's pod before running a command")
	cmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Environment variables to be substituted in helm's pod (WITHOUT values). Values will be substituted from exported env on host")
	cmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", copyRepoDefault, "Copy existing helm repositories to helm pod")
	cmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "A list of helm repository aliases which should be updated before running a command. Applicable only if --copy-repo set to true. All repositories will be updated if not specified")
	cmd.Flags().StringSliceVarP(&opts.Files, "copy", "c", []string{}, "A map of files/directories which should be copied from host to container. Can be specified multiple times. Example: -c /path_on_host/values.yaml:/path_in_container/values.yaml")
	cmd.Flags().IntVar(&opts.CopyAttempts, "copy-attempts", 3, "Attempts count in each copy action (in copy-repo and copy flags). If your connection to k8s api is no stable, you can try to increase the attempts")
	cmd.Flags().IntVar(&opts.UpdateRepoAttempts, "update-repo-attempts", 3, "Attempts count in each helm update repo action. Applicable only if copy-repo set to true")
}
