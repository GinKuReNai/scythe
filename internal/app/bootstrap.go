package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/GinKuReNai/scythe/internal/cache/sqlite"
	"github.com/GinKuReNai/scythe/internal/config"
	"github.com/GinKuReNai/scythe/internal/decision/jev"
	"github.com/GinKuReNai/scythe/internal/scan"
	"github.com/GinKuReNai/scythe/internal/typescript"
)

type App struct {
	Logger    *slog.Logger
	LookupEnv func(string) (string, bool)
}

func New(logger *slog.Logger) *App { return &App{logger, os.LookupEnv} }
func (a *App) Config(root string, overrides config.Overrides) (config.Config, error) {
	return config.LoadWithOverrides(root, a.LookupEnv, overrides)
}
func (c *App) CachePath(cfg config.Config) (string, error) {
	directory := cfg.CacheDir
	if directory == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("locate user cache directory: %w", err)
		}
		directory = filepath.Join(base, "deadcode")
	}
	return filepath.Join(directory, "cache.db"), nil
}
func (a *App) Scanner(ctx context.Context, cfg config.Config, offline bool) (*scan.Service, func() error, error) {
	script := cfg.Analyzer
	if script == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, nil, err
		}
		script = filepath.Join(filepath.Dir(executable), "analyzer", "dist", "index.js")
		if _, err = os.Stat(script); err != nil {
			script = filepath.Join("analyzer", "typescript", "dist", "index.js")
		}
	}
	if _, err := os.Stat(script); err != nil {
		return nil, nil, fmt.Errorf("built TypeScript analyzer not found; run make build or set --analyzer")
	}
	var engine scan.DecisionEngine
	key, _ := a.LookupEnv("JEV_API_KEY")
	if !offline && key != "" {
		engine = jev.NewClient(&http.Client{Timeout: time.Duration(cfg.Jev.TimeoutSeconds) * time.Second}, "", cfg.Jev.Model, key)
	}
	if !offline && key == "" {
		a.Logger.InfoContext(ctx, "JEV_API_KEY is missing; ambiguous candidates will be reviewed locally")
	}
	var store scan.Cache
	closeStore := func() error { return nil }
	// Offline means no network; cached decisions remain usable unless disabled.
	if cfg.Cache.Enabled {
		path, err := a.CachePath(cfg)
		if err != nil {
			return nil, nil, err
		}
		db, err := sqlite.Open(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		store = db
		closeStore = db.Close
	}
	analyzer := &typescript.Analyzer{Script: script, Exclude: cfg.Analysis.Exclude, Secret: key}
	return scan.NewService(analyzer, engine, store, cfg.Policy, cfg.Jev.Model, jev.QuestionSetVersion, cfg.Jev.Concurrency), closeStore, nil
}
func (a *App) Clean(ctx context.Context, cfg config.Config) error {
	path, err := a.CachePath(cfg)
	if err != nil {
		return err
	}
	db, err := sqlite.Open(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Clean(ctx)
}
