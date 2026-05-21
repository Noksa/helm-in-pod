package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/logz"
)

func newDaemonExecCmd() *cobra.Command {
	opts := cmdoptions.DaemonOptions{}
	execCmd := &cobra.Command{
		Use:   "exec [flags] -- <command_to_run>",
		Short: "Execute a command in a running daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			opts.Name, err = getDaemonName(opts.Name)
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return fmt.Errorf("specify command to run")
			}

			// Make retry loops respect the command context (timeout / signals)
			internal.UseCommandContext(cmd.Context())

			logz.Host().Debug().Msgf("Looking for %s daemon", color.CyanString(opts.Name))
			pod, err := internal.Pod().GetDaemonPod(opts.Name)
			if err != nil {
				return err
			}
			logz.Host().Info().Msgf("Found %s daemon", color.CyanString(pod.Name))

			homeDirectory := pod.Annotations[hipconsts.AnnotationHomeDirectory]
			if homeDirectory == "" {
				return fmt.Errorf("daemon pod missing home-directory annotation")
			}

			helmFound := pod.Annotations[hipconsts.AnnotationHelmFound] == "true"

			if helmFound && (opts.CopyRepo || len(opts.UpdateRepo) > 0 || opts.UpdateAllRepos) {
				if opts.CopyAttempts < 1 {
					return fmt.Errorf("copy-attempts value can't be less 1")
				}
				if opts.UpdateRepoAttempts < 1 {
					return fmt.Errorf("update-repo-attempts value can't be less 1")
				}

				switch {
				case opts.CopyRepo:
					err = internal.Pod().SyncHelmRepositories(pod, opts.ExecOptions, homeDirectory, false)
					if err != nil {
						return err
					}
				case opts.UpdateAllRepos:
					// Update all repos without copying
					opts.UpdateRepo = []string{}
					err = internal.Pod().UpdateHelmRepositories(pod, opts.ExecOptions)
					if err != nil {
						return err
					}
				case len(opts.UpdateRepo) > 0:
					err = internal.Pod().UpdateHelmRepositories(pod, opts.ExecOptions)
					if err != nil {
						return err
					}
				}
			}

			if len(opts.Files) > 0 {
				opts.ParseFileMappings()
				err = internal.Pod().CopyUserFiles(pod, opts.ExecOptions, expand, opts.Clean)
				if err != nil {
					return err
				}
			}

			// Load environment variables from files
			if err := opts.ParseEnvFiles(); err != nil {
				return err
			}

			cmdToUse := strings.Join(args, " ")
			timeout := viper.GetDuration("timeout")
			if timeout == 0 {
				timeout = time.Hour * 2
			}
			execErr := internal.Pod().ExecuteCommandInDaemon(cmd.Context(), pod, cmdToUse, homeDirectory, timeout, opts.ExecOptions)

			// Copy files from pod to host after command execution
			if len(opts.CopyFrom) > 0 {
				copyFromMap, parseErr := parseCopyFromMappings(opts.CopyFrom)
				if parseErr != nil {
					if execErr != nil {
						return execErr
					}
					return parseErr
				}
				var copyErrors []error
				for podPath, hostPath := range copyFromMap {
					expanded, expandErr := expand(hostPath)
					if expandErr != nil {
						copyErrors = append(copyErrors, expandErr)
						continue
					}
					if copyErr := internal.Pod().CopyFileFromPod(pod, podPath, expanded, opts.CopyAttempts); copyErr != nil {
						copyErrors = append(copyErrors, copyErr)
					}
				}
				if len(copyErrors) > 0 {
					if execErr != nil {
						return execErr
					}
					return copyErrors[0]
				}
			}

			return execErr
		},
	}
	execCmd.Flags().StringVar(&opts.Name, "name", "", "Daemon name (required)")
	execCmd.Flags().BoolVar(&opts.UpdateAllRepos, "update-all-repos", false, "Update all helm repositories without copying them")
	execCmd.Flags().StringSliceVar(&opts.Clean, "clean", []string{}, "Paths to delete in the pod before copying files")
	addRuntimeFlags(execCmd, &opts.ExecOptions, false)
	return execCmd
}
