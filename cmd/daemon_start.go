package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"helm.sh/helm/v4/pkg/cli"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
	"github.com/noksa/helm-in-pod/internal/logz"
)

func newDaemonStartCmd() *cobra.Command {
	opts := cmdoptions.DaemonOptions{}
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a persistent daemon pod for repeated command execution",
		Long: `Create a long-running pod that stays alive for executing multiple commands without pod recreation overhead.

Helm repositories are synced from the host automatically (disable with --copy-repo=false).
Use 'daemon exec' to run commands and 'daemon stop' to tear down the pod.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			opts.Name, err = getDaemonName(opts.Name)
			if err != nil {
				return err
			}
			if opts.CopyAttempts < 1 {
				return fmt.Errorf("copy-attempts value can't be less 1")
			}
			if opts.UpdateRepoAttempts < 1 {
				return fmt.Errorf("update-repo-attempts value can't be less 1")
			}

			internal.UseCommandContext(cmd.Context())

			timeout := viper.GetDuration("timeout")
			if timeout == 0 {
				timeout = time.Hour * 2
			}
			opts.Timeout = timeout + time.Minute*10

			if opts.Labels == nil {
				opts.Labels = map[string]string{}
			}
			opts.Labels["daemon"] = opts.Name

			// Handle dry-run: print pod spec and exit
			if opts.DryRun {
				return internal.Pod().PrintPodSpecYAML(opts.ExecOptions, true)
			}

			opts.ParseFileMappings()

			// Load environment variables from files
			if err := opts.ParseEnvFiles(); err != nil {
				return err
			}

			err = internal.Namespace().PrepareNs()
			if err != nil {
				return err
			}

			pod, err := internal.Pod().CreateDaemonPod(opts)
			if err != nil {
				return err
			}

			// Build bundle: user files + repositories.yaml (if applicable).
			// CopyFilesBundleWithBootInfo collects home dir, user, and helm version in
			// the same exec call, replacing the separate GetPodUserInfo + IsHelm4 round trips.
			bundle := make([]helmtar.BundleEntry, 0, len(opts.FilesAsMap)+1)
			for src, dest := range opts.FilesAsMap {
				expandedSrc, expandErr := expand(src)
				if expandErr != nil {
					return expandErr
				}
				bundle = append(bundle, helmtar.BundleEntry{SrcPath: expandedSrc, DestPath: dest})
			}

			repoConfigStaged := false
			if opts.CopyRepo {
				settings := cli.New()
				_, statErr := os.Stat(settings.RepositoryConfig)
				if statErr == nil {
					bundle = append(bundle, helmtar.BundleEntry{SrcPath: settings.RepositoryConfig, DestPath: hipconsts.StagedRepoConfigPath})
					repoConfigStaged = true
				} else if !errors.Is(statErr, os.ErrNotExist) {
					return statErr
				}
			}

			bootInfo, err := internal.Pod().CopyFilesBundleWithBootInfo(pod, bundle, nil, opts.CopyAttempts, repoConfigStaged)
			if err != nil {
				return err
			}

			if !bootInfo.HelmFound {
				logz.Pod().Warn().Msg("helm is not installed in the image, all helm prerequisites will be skipped")
			}

			if opts.CopyRepo && bootInfo.HelmFound && repoConfigStaged {
				err = internal.Pod().SyncHelmRepositories(pod, opts.ExecOptions, bootInfo.HomeDirectory, bootInfo.IsHelm4, true)
				if err != nil {
					return err
				}
			}

			annotations := map[string]string{
				hipconsts.AnnotationHomeDirectory: bootInfo.HomeDirectory,
				hipconsts.AnnotationHelmFound:     fmt.Sprintf("%v", bootInfo.HelmFound),
			}
			if bootInfo.HelmFound {
				annotations[hipconsts.AnnotationHelm4] = fmt.Sprintf("%v", bootInfo.IsHelm4)
			}
			err = internal.Pod().AnnotatePod(pod, annotations)
			if err != nil {
				return err
			}

			logz.Host().Info().Msgf("Daemon pod '%s' started successfully", color.CyanString(pod.Name))
			return nil
		},
	}

	startCmd.Flags().StringVar(&opts.Name, "name", "", "Daemon name (required)")
	startCmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "Force recreate daemon pod if it already exists")
	addExecOptionsFlags(startCmd, &opts.ExecOptions)

	return startCmd
}
