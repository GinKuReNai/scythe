package cli

import (
	"fmt"

	"github.com/GinKuReNai/scythe/internal/config"
	"github.com/spf13/cobra"
)

func NewCacheCommand(deps Dependencies) *cobra.Command {
	cmd := &cobra.Command{Use: "cache", Short: "Manage cached model decisions"}
	cmd.AddCommand(&cobra.Command{Use: "clean", Short: "Remove all cached decisions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := deps.LoadConfig(".", config.Overrides{})
		if err != nil {
			return err
		}
		if err = deps.Clean(cmd.Context(), cfg); err != nil {
			return fmt.Errorf("clean cache: %w", err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Decision cache cleared.")
		return err
	}})
	return cmd
}
