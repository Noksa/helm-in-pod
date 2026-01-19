package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDaemonExecCmd() *cobra.Command {
	opts := cmdoptions.DaemonOptions{}
	execCmd := &cobra.Command{
		Use:   "exec [flags] -- <command_to_run>",
		Short: "Execute command in a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Name == "" {
				return fmt.Errorf("--name is required")
			}
			if len(args) == 0 {
				return fmt.Errorf("specify command to run")
			}

			pod, err := internal.Pod.GetDaemonPod(opts.Name)
			if err != nil {
				return err
			}

			homeDirectory := pod.Annotations[internal.AnnotationHomeDirectory]
			if homeDirectory == "" {
				return fmt.Errorf("daemon pod missing home-directory annotation")
			}

			helmFound := pod.Annotations[internal.AnnotationHelmFound] == "true"
			isHelm4 := pod.Annotations[internal.AnnotationHelm4] == "true"

			if helmFound && (opts.CopyRepo || len(opts.UpdateRepo) > 0) {
				if opts.CopyAttempts < 1 {
					return fmt.Errorf("copy-attempts value can't be less 1")
				}
				if opts.UpdateRepoAttempts < 1 {
					return fmt.Errorf("update-repo-attempts value can't be less 1")
				}

				if opts.CopyRepo {
					err = internal.Pod.SyncHelmRepositories(pod, opts.ExecOptions, homeDirectory, isHelm4)
					if err != nil {
						return err
					}
				} else if len(opts.UpdateRepo) > 0 {
					err = internal.Pod.UpdateHelmRepositories(pod, opts.ExecOptions, isHelm4)
					if err != nil {
						return err
					}
				}
			}

			if len(opts.Files) > 0 {
				opts.FilesAsMap = map[string]string{}
				for _, val := range opts.Files {
					entries := strings.SplitSeq(val, ",")
					for v := range entries {
						splitted := strings.Split(v, ":")
						opts.FilesAsMap[splitted[0]] = splitted[1]
					}
				}
				err = internal.Pod.CopyUserFiles(pod, opts.ExecOptions, expand)
				if err != nil {
					return err
				}
			}

			cmdToUse := strings.Join(args, " ")
			timeout := viper.GetDuration("timeout")
			if timeout == 0 {
				timeout = time.Hour * 2
			}
			return internal.Pod.ExecuteCommandInDaemon(cmd.Context(), pod, cmdToUse, homeDirectory, timeout, opts.ExecOptions)
		},
	}
	execCmd.Flags().StringVar(&opts.Name, "name", "", "Daemon name (required)")
	addExecOptionsFlags(execCmd, &opts.ExecOptions)
	return execCmd
}
