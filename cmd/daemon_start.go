package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/noksa/helm-in-pod/internal/cmdoptions"
	"github.com/noksa/helm-in-pod/internal/helpers"
	"github.com/noksa/helm-in-pod/internal/logz"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newDaemonStartCmd() *cobra.Command {
	opts := cmdoptions.DaemonOptions{}
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Name == "" {
				return fmt.Errorf("--name is required")
			}
			if opts.CopyAttempts < 1 {
				return fmt.Errorf("copy-attempts value can't be less 1")
			}
			if opts.UpdateRepoAttempts < 1 {
				return fmt.Errorf("update-repo-attempts value can't be less 1")
			}

			timeout := viper.GetDuration("timeout")
			if timeout == 0 {
				timeout = time.Hour * 2
			}
			opts.Timeout = timeout + time.Minute*10

			if opts.Labels == nil {
				opts.Labels = map[string]string{}
			}
			opts.Labels["daemon"] = opts.Name

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

			err := internal.Namespace.PrepareNs()
			if err != nil {
				return err
			}

			pod, err := internal.Pod.CreateDaemonPod(opts)
			if err != nil {
				return err
			}

			userInfo, err := internal.Pod.GetPodUserInfo(pod)
			if err != nil {
				return err
			}

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
				log.Warnf("%v helm is not installed in the image, all helm prerequisites will be skipped", logz.LogPod())
			}

			if opts.CopyRepo && helmFound {
				err = internal.Pod.SyncHelmRepositories(pod, opts.ExecOptions, userInfo.HomeDirectory, isHelm4)
				if err != nil {
					return err
				}
			}

			err = internal.Pod.CopyUserFiles(pod, opts.ExecOptions, expand)
			if err != nil {
				return err
			}

			// Annotate pod with user info and helm version
			annotations := map[string]string{
				internal.AnnotationHomeDirectory: userInfo.HomeDirectory,
				internal.AnnotationHelmFound:     fmt.Sprintf("%v", helmFound),
			}
			if helmFound {
				annotations[internal.AnnotationHelm4] = fmt.Sprintf("%v", isHelm4)
			}
			err = internal.Pod.AnnotatePod(pod, annotations)
			if err != nil {
				return err
			}

			log.Infof("Daemon pod '%s' started successfully", color.CyanString(pod.Name))
			return nil
		},
	}

	startCmd.Flags().StringVar(&opts.Name, "name", "", "Daemon name (required)")
	addExecOptionsFlags(startCmd, &opts.ExecOptions)

	return startCmd
}
