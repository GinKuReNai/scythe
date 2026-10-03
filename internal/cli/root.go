package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/GinKuReNai/scythe/internal/config"
	"github.com/GinKuReNai/scythe/internal/report"
	"github.com/GinKuReNai/scythe/internal/scan"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Version    string
	LoadConfig func(string, config.Overrides) (config.Config, error)
	Scanner    func(context.Context, config.Config, bool) (*scan.Service, func() error, error)
	Clean      func(context.Context, config.Config) error
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	root := &cobra.Command{Use: "deadcode", Short: "Find dead TypeScript code using compiler evidence and conservative judgments", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(NewScanCommand(deps), NewExplainCommand(deps), NewCacheCommand(deps), NewVersionCommand(deps.Version))
	return root
}

type options struct {
	format, model, analyzer string
	concurrency             int
	offline, noCache        bool
}

func (o *options) flags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&o.format, "format", "", "Output format: text or json")
	f.StringVar(&o.model, "model", "", "Jev model")
	f.IntVar(&o.concurrency, "concurrency", 0, "Maximum concurrent decisions (1-64)")
	f.StringVar(&o.analyzer, "analyzer", "", "Path to built TypeScript analyzer script")
	f.BoolVar(&o.offline, "offline", false, "Disable Jev HTTP calls")
	f.BoolVar(&o.noCache, "no-cache", false, "Disable decision caching")
}
func (o options) config(cmd *cobra.Command, deps Dependencies, path string) (config.Config, error) {

	overrides := config.Overrides{}
	if cmd.Flags().Changed("format") {
		overrides.Format = &o.format
	}
	if cmd.Flags().Changed("model") {
		overrides.Model = &o.model
	}
	if cmd.Flags().Changed("analyzer") {
		overrides.Analyzer = &o.analyzer
	}
	if cmd.Flags().Changed("concurrency") {
		overrides.Concurrency = &o.concurrency
	}
	if o.noCache {
		enabled := false
		overrides.CacheEnabled = &enabled
	}
	return deps.LoadConfig(path, overrides)
}
func executeScan(cmd *cobra.Command, deps Dependencies, o options, path, symbol string) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	cfg, err := o.config(cmd, deps, path)
	if err != nil {
		return err
	}
	service, closeService, err := deps.Scanner(cmd.Context(), cfg, o.offline)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeService(); err == nil && closeErr != nil {
			err = fmt.Errorf("close decision cache: %w", closeErr)
		}
	}()

	var result *scan.Result
	if symbol == "" {
		result, err = service.Scan(cmd.Context(), path)
	} else {
		result, err = service.Explain(cmd.Context(), path, symbol)
	}
	if err != nil {
		return err
	}
	var reporter report.Reporter = report.Text{Explain: symbol != ""}
	if cfg.Format == "json" {
		reporter = report.JSON{}
	}
	return reporter.Write(cmd.OutOrStdout(), result)
}
