package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDaemonExecCmd() *cobra.Command {
	opts := cmdoptions.DaemonOptions{}
	execCmd := &cobra.Command{
		Use:   "exec [flags] -- <command_to_run>",
		Short: "Execute command in a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			opts.Name, err = getDaemonName(opts.Name)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return fmt.Errorf("specify command to run")
			}
			log.Debugf("%s Looking for %s daemon", logz.LogHost(), color.CyanString(opts.Name))
			pod, err := internal.Pod.GetDaemonPod(opts.Name)
			if err != nil {
				return err
			}
			log.Infof("%s Found %s daemon", logz.LogHost(), color.CyanString(pod.Name))

			homeDirectory := pod.Annotations[hipconsts.AnnotationHomeDirectory]
			if homeDirectory == "" {
				return fmt.Errorf("daemon pod missing home-directory annotation")
			}

			helmFound := pod.Annotations[hipconsts.AnnotationHelmFound] == "true"
			isHelm4 := pod.Annotations[hipconsts.AnnotationHelm4] == "true"

			if helmFound && (opts.CopyRepo || len(opts.UpdateRepo) > 0 || opts.UpdateAllRepos) {
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
				} else if opts.UpdateAllRepos {
					// Update all repos without copying
					opts.UpdateRepo = []string{}
					err = internal.Pod.UpdateHelmRepositories(pod, opts.ExecOptions, isHelm4)
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
				err = internal.Pod.CopyUserFiles(pod, opts.ExecOptions, expand, opts.Clean)
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
	execCmd.Flags().BoolVar(&opts.UpdateAllRepos, "update-all-repos", false, "Update all helm repositories without copying them")
	execCmd.Flags().StringSliceVar(&opts.Clean, "clean", []string{}, "Paths to delete before copying files")
	addRuntimeFlags(execCmd, &opts.ExecOptions, false)
	return execCmd
}
