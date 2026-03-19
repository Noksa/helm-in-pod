package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/noksa/helm-in-pod/internal"
	"github.com/spf13/cobra"
)

func newDaemonListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all daemon pods",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := internal.Pod().ListDaemonPods()
			if err != nil {
				return err
			}

			if len(infos) == 0 {
				fmt.Println("No daemon pods found")
				return nil
			}

			table := cyberTable(os.Stdout)
			table.Header([]string{"NAME", "POD", "PHASE", "NODE", "AGE", "HELM", "IMAGE"})
			for _, info := range infos {
				helmStr := "no"
				if info.HelmFound {
					ver := "3"
					if info.IsHelm4 {
						ver = "4"
					}
					helmStr = fmt.Sprintf("v%s", ver)
				}
				_ = table.Append([]string{
					color.CyanString(info.Name),
					info.PodName,
					colorPhase(string(info.Phase)),
					info.Node,
					formatDuration(info.Age),
					helmStr,
					info.Image,
				})
			}
			_ = table.Render()
			return nil
		},
	}
	return listCmd
}
