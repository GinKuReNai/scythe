package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/GinKuReNai/scythe/internal/app"
	"github.com/GinKuReNai/scythe/internal/cli"
)

var version = "0.1.0-dev"

func main() { os.Exit(run()) }
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application := app.New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	command := cli.NewRootCommand(cli.Dependencies{Version: version, LoadConfig: application.Config, Scanner: application.Scanner, Clean: application.Clean})
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if ctx.Err() != nil {
			return 130
		}
		return 1
	}
	return 0
}
