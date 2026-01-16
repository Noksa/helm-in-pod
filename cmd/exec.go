package cmd

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helpers"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/multierr"
)

func newExecCmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:     "exec [flags] -- <command_to_run>",
		Aliases: []string{"run"},
		Short:   "Executes commands in a pod inside k8s cluster",
	}
	opts := cmdoptions.ExecOptions{}
	execCmd.Flags().Int64Var(&opts.RunAsUser, "run-as-user", -1, "Run as user ID to be set in security context. Omitted if not specified and default from an image is used")
	execCmd.Flags().Int64Var(&opts.RunAsGroup, "run-as-group", -1, "Run as group ID to be set in security context. Omitted if not specified and default from an image is used")
	execCmd.Flags().StringToStringVar(&opts.Labels, "labels", map[string]string{}, "Additional labels to be set on a pod")
	execCmd.Flags().StringToStringVar(&opts.Annotations, "annotations", map[string]string{}, "Additional annotations to be set on a pod")
	execCmd.Flags().StringVar(&opts.Cpu, "cpu", "1100m", "Pod's cpu request/limit")
	execCmd.Flags().StringVar(&opts.Memory, "memory", "500Mi", "Pod's memory request/limit")
	execCmd.Flags().BoolVar(&opts.HostNetwork, "host-network", false, "Use host network in a pod")
	execCmd.Flags().StringSliceVar(&opts.Tolerations, "tolerations", []string{}, "Pod's tolerations in format key=value:effect:operator. Examples: '::Exists' (all taints), 'key=::Exists' (key with any effect), 'key=:NoSchedule:Exists', 'key=value:NoSchedule:Equal'")
	execCmd.Flags().StringToStringVar(&opts.NodeSelector, "node-selector", map[string]string{}, "Pod's node selectors. Examples: 'node-role.kubernetes.io/control-plane=\"\"', 'disktype=ssd'")
	execCmd.Flags().StringToStringVarP(&opts.Env, "env", "e", map[string]string{}, "Environment variables to be set in helm's pod before running a command")
	execCmd.Flags().StringSliceVarP(&opts.SubstEnv, "subst-env", "s", []string{}, "Environment variables to be substituted in helm's pod (WITHOUT values). Values will be substituted from exported env on host")
	execCmd.Flags().StringVar(&opts.ImagePullSecret, "image-pull-secret", "", "Specify an image pull secret which should be used to pull --image from private repository")
	execCmd.Flags().BoolVar(&opts.CopyRepo, "copy-repo", true, "Copy existing helm repositories to helm pod")
	execCmd.Flags().StringVar(&opts.PullPolicy, "pull-policy", "IfNotPresent", "Image pull policy to use in helm pod")
	execCmd.Flags().StringSliceVar(&opts.UpdateRepo, "update-repo", []string{}, "A list of helm repository aliases which should be updated before running a command. Applicable only if --copy-repo set to true. All repositories will be updated if not specified")
	execCmd.Flags().StringVarP(&opts.Image, "image", "i", "docker.io/noksa/kubectl-helm:v1.34.2-v4.0.4", "An image which will be used")
	execCmd.Flags().StringSliceVarP(&opts.Files, "copy", "c", []string{}, "A map of files/directories which should be copied from host to container. Can be specified multiple times. Example: -c /path_on_host/values.yaml:/path_in_container/values.yaml")
	execCmd.Flags().IntVar(&opts.CopyAttempts, "copy-attempts", 3, "Attempts count in each copy action (in copy-repo and copy flags). If your connection to k8s api is no stable, you can try to increase the attempts")
	execCmd.Flags().IntVar(&opts.UpdateRepoAttempts, "update-repo-attempts", 3, "Attempts count in each helm update repo action. Applicable only if copy-repo set to true")
	execCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify command to run. Run `helm in-pod exec --help` to check available options")
		}
		if opts.CopyAttempts < 1 {
			return fmt.Errorf("copy-attempts value can't be less 1")
		}
		if opts.UpdateRepoAttempts < 1 {
			return fmt.Errorf("update-repo-attempts value can't be less 1")
		}

		timeout := viper.GetDuration("timeout")
		opts.Timeout = timeout + time.Minute*10

		var mErr error
		defer multierr.AppendInvoke(&mErr, multierr.Invoke(func() error {
			return internal.Pod.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
		}))

		// Parse file mappings
		if len(opts.Files) > 0 {
			opts.FilesAsMap = map[string]string{}
			for _, val := range opts.Files {
				entries := strings.SplitSeq(val, ",")
				for v := range entries {
					splitted := strings.Split(v, ":")
					opts.FilesAsMap[splitted[0]] = splitted[1]
				}
			}
		}

		// Prepare namespace and create pod
		err := internal.Namespace.PrepareNs()
		if err != nil {
			return err
		}

		pod, err := internal.Pod.CreateHelmPod(opts)
		if err != nil {
			return err
		}

		// Get pod user info
		userInfo, err := getPodUserInfo(pod)
		if err != nil {
			return err
		}

		// Check if helm is installed
		helmFound := false
		isHelm4, err := helpers.IsHelm4(pod.Name, pod.Namespace, opts.Image)
		if err != nil {
			if !strings.Contains(err.Error(), "helm is not installed") {
				return err
			}
		} else {
			helmFound = true
		}

		if !helmFound {
			log.Warnf("%v helm is not installed in the image, all helm prerequisites will be skipped. If the passed command contains helm calls, it will fail", logz.LogPod())
		}

		// Sync helm repositories if needed
		if opts.CopyRepo && helmFound {
			err = syncHelmRepositories(pod, opts, userInfo.homeDirectory, isHelm4)
			if err != nil {
				return err
			}
		}

		// Copy user files
		err = copyUserFiles(pod, opts)
		if err != nil {
			return err
		}

		// Execute command
		cmdToUse := strings.Join(args, " ")
		return executeCommand(cmd.Context(), pod, cmdToUse, userInfo.homeDirectory, opts)
	}
	return execCmd
}

func expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}
