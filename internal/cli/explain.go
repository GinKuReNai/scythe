package cli

import "github.com/spf13/cobra"

func NewExplainCommand(deps Dependencies) *cobra.Command {
	var o options
	var path string
	cmd := &cobra.Command{Use: "explain <symbol>", Short: "Re-analyze and show a candidate's evidence and decision", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return executeScan(cmd, deps, o, path, args[0]) }}
	o.flags(cmd)
	cmd.Flags().StringVar(&path, "path", ".", "TypeScript project directory")
	return cmd
}
