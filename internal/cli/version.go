package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewVersionCommand(version string) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print the CLI version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "deadcode %s\n", version)
		return err
	}}
}
