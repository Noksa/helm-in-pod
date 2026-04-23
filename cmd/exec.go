package cmd

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/logz"
)

func newExecCmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:     "exec [flags] -- <command_to_run>",
		Aliases: []string{"run"},
		Short:   "Execute a command in a one-shot pod",
		Long: `Create a temporary pod in the cluster, execute the specified command, and clean up.

Helm repositories are synced from the host automatically (disable with --copy-repo=false).
Files can be copied to the pod before execution and back to the host after.
The pod is deleted after the command completes, even on failure.`,
	}
	opts := cmdoptions.ExecOptions{}
	addExecOptionsFlags(execCmd, &opts)
	execCmd.RunE = func(cmd *cobra.Command, args []string) (returnErr error) {
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

		// Handle dry-run: print pod spec and exit
		if opts.DryRun {
			return internal.Pod().PrintPodSpecYAML(opts, false)
		}

		defer func() {
			cleanupErr := internal.Pod().DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			if cleanupErr != nil && returnErr == nil {
				returnErr = cleanupErr
			}
		}()

		// Parse file mappings
		opts.ParseFileMappings()

		// Prepare namespace and create pod
		err := internal.Namespace().PrepareNs()
		if err != nil {
			return err
		}

		pod, err := internal.Pod().CreateHelmPod(opts)
		if err != nil {
			return err
		}

		cmdToUse := strings.Join(args, " ")

		bundle := make([]helmtar.BundleEntry, 0, len(opts.FilesAsMap))
		for src, dest := range opts.FilesAsMap {
			expandedSrc, expandErr := expand(src)
			if expandErr != nil {
				return expandErr
			}
			bundle = append(bundle, helmtar.BundleEntry{SrcPath: expandedSrc, DestPath: dest})
		}

		bootInfo, err := internal.Pod().CopyFilesBundleWithBootInfo(pod, bundle, nil, opts.CopyAttempts)
		if err != nil {
			return err
		}

		if !bootInfo.HelmFound {
			logz.Pod().Warn().Msg("helm is not installed in the image, all helm prerequisites will be skipped. If the passed command contains helm calls, it will fail")
		}

		if opts.CopyRepo && bootInfo.HelmFound {
			err = internal.Pod().SyncHelmRepositories(pod, opts, bootInfo.HomeDirectory, bootInfo.IsHelm4)
			if err != nil {
				return err
			}
		}

		execErr := internal.Pod().ExecuteCommand(cmd.Context(), pod, cmdToUse, bootInfo.HomeDirectory, opts)

		// Copy files from pod to host (even if command failed, user may want artifacts)
		if len(opts.CopyFrom) > 0 {
			copyFromMap, parseErr := parseCopyFromMappings(opts.CopyFrom)
			if parseErr != nil {
				internal.Pod().SignalCopyDone(pod)
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
			// Signal the pod that copy is done so it can exit
			internal.Pod().SignalCopyDone(pod)
			if len(copyErrors) > 0 {
				if execErr != nil {
					return execErr
				}
				return copyErrors[0]
			}
		}

		return execErr
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
