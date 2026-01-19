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
	addExecOptionsFlags(execCmd, &opts)
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
		userInfo, err := internal.Pod.GetPodUserInfo(pod)
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
			err = internal.Pod.SyncHelmRepositories(pod, opts, userInfo.HomeDirectory, isHelm4)
			if err != nil {
				return err
			}
		}

		// Copy user files
		err = internal.Pod.CopyUserFiles(pod, opts, expand)
		if err != nil {
			return err
		}

		// Execute command
		cmdToUse := strings.Join(args, " ")
		return internal.Pod.ExecuteCommand(cmd.Context(), pod, cmdToUse, userInfo.HomeDirectory, opts)
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
