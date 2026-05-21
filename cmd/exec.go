package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"helm.sh/helm/v4/pkg/cli"

	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helmtar"
	"github.com/noksa/helm-in-pod/internal/hipconsts"
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

		// Make all retry loops inside the managers respect this command's context
		// (so --timeout and signals cancel backoff sleeps).
		internal.UseCommandContext(cmd.Context())

		timeout := viper.GetDuration("timeout")
		opts.Timeout = timeout + time.Minute*10

		// Handle dry-run: print pod spec and exit
		if opts.DryRun {
			return internal.Pod().PrintPodSpecYAML(opts, false)
		}

		defer func() {
			// On signal interrupt, CreateHelmPod's signal handler already deleted
			// the pod — skip the redundant call. On timeout or normal exit, run
			// cleanup; if the command context is dead (timeout), use a fresh
			// background context so the API call doesn't fail with "context canceled".
			if errors.Is(cmd.Context().Err(), context.Canceled) {
				return
			}
			// --keep-pod: leave the pod alive so the user can inspect it.
			// The pod is removed on the next exec or purge run.
			if opts.KeepPod {
				return
			}
			pod := internal.Pod()
			if cmd.Context().Err() != nil {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				pod = pod.WithContext(cleanupCtx)
			}
			cleanupErr := pod.DeleteHelmPods(opts, cmdoptions.PurgeOptions{All: false})
			if cleanupErr != nil && returnErr == nil {
				returnErr = cleanupErr
			}
		}()

		// Parse file mappings
		opts.ParseFileMappings()

		// Load environment variables from files
		if err := opts.ParseEnvFiles(); err != nil {
			return err
		}

		// Prepare namespace and create pod
		err := internal.Namespace().PrepareNs()
		if err != nil {
			return err
		}

		pod, err := internal.Pod().CreateHelmPod(opts)
		if err != nil {
			return err
		}
		if opts.KeepPod {
			logz.Host().Info().Msgf("Pod %v will be kept after exec — inspect with: kubectl exec -n %v %v -- sh", pod.Name, pod.Namespace, pod.Name)
		}

		cmdToUse := strings.Join(args, " ")

		// Generate the wrapped script
		tempScriptFile, err := os.CreateTemp("", hipconsts.Namespace)
		if err != nil {
			return err
		}
		defer func() {
			_ = tempScriptFile.Close()
			_ = os.RemoveAll(tempScriptFile.Name())
		}()
		if err := os.Chmod(tempScriptFile.Name(), os.ModePerm); err != nil {
			return err
		}
		for _, s := range []string{"#!/bin/sh\nset -eu\n", cmdToUse, "\n"} {
			if _, wErr := tempScriptFile.WriteString(s); wErr != nil {
				return wErr
			}
		}
		_ = tempScriptFile.Close()

		// Build bundle: user files + wrapped script + repositories.yaml
		bundle := make([]helmtar.BundleEntry, 0, len(opts.FilesAsMap)+2)
		for src, dest := range opts.FilesAsMap {
			expandedSrc, expandErr := expand(src)
			if expandErr != nil {
				return expandErr
			}
			bundle = append(bundle, helmtar.BundleEntry{SrcPath: expandedSrc, DestPath: dest})
		}
		bundle = append(bundle, helmtar.BundleEntry{SrcPath: tempScriptFile.Name(), DestPath: hipconsts.StagedScriptPath})

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
			logz.Pod().Warn().Msg("helm is not installed in the image, all helm prerequisites will be skipped. If the passed command contains helm calls, it will fail")
		}

		if opts.CopyRepo && bootInfo.HelmFound && repoConfigStaged {
			err = internal.Pod().SyncHelmRepositories(pod, opts, bootInfo.HomeDirectory, true)
			if err != nil {
				return err
			}
		}

		execErr := internal.Pod().ExecuteCommand(cmd.Context(), pod, cmdToUse, opts, true)

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
