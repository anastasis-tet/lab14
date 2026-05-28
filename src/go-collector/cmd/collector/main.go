package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/anastasis-tet/lab14/src/go-collector/internal/app"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/config"
	"github.com/anastasis-tet/lab14/src/go-collector/internal/logging"
)

func main() {
	logger := logging.New(os.Stdout)
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("collector stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
