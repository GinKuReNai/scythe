package cli

import "github.com/spf13/cobra"

func NewScanCommand(deps Dependencies) *cobra.Command {
	var o options
	cmd := &cobra.Command{Use: "scan [path]", Short: "Scan a TypeScript project without modifying source", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		return executeScan(cmd, deps, o, path, "")
	}}
	o.flags(cmd)
	return cmd
}
