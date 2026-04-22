package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/noksa/helm-in-pod/internal"
)

func newDaemonStatusCmd() *cobra.Command {
	var name string
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of a daemon pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			name, err = getDaemonName(name)
			if err != nil {
				return err
			}

			info, err := internal.Pod().GetDaemonStatus(name)
			if err != nil {
				return err
			}

			helmStr := "not found"
			if info.HelmFound {
				ver := "3"
				if info.IsHelm4 {
					ver = "4"
				}
				helmStr = fmt.Sprintf("v%s", ver)
			}

			rows := [][]string{
				{"Name", color.CyanString(info.Name)},
				{"Pod", info.PodName},
				{"Phase", colorPhase(string(info.Phase))},
				{"Node", info.Node},
				{"Age", formatDuration(info.Age)},
				{"Image", info.Image},
				{"Helm", helmStr},
			}
			if info.HomeDir != "" {
				rows = append(rows, []string{"Home Dir", info.HomeDir})
			}

			table := cyberTable(os.Stdout)
			table.Header([]string{"PROPERTY", "VALUE"})
			_ = table.Bulk(rows)
			_ = table.Render()
			return nil
		},
	}
	statusCmd.Flags().StringVar(&name, "name", "", "Daemon name (required)")
	return statusCmd
}

func colorPhase(phase string) string {
	switch phase {
	case "Running":
		return color.GreenString(phase)
	case "Pending":
		return color.YellowString(phase)
	default:
		return color.RedString(phase)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
