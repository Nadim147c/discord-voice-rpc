package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM) // only works on linux
	defer cancel()

	var level log.Level // zero value is info
	if *debug {
		level = log.DebugLevel
	}

	handler := log.NewWithOptions(os.Stderr, log.Options{
		TimeFunction: nil,
		Level:        level,
	})
	slog.SetDefault(slog.New(handler))

	client := NewClient()
	if err := client.Listen(ctx); err != nil &&
		!errors.Is(context.Canceled, err) &&
		!errors.Is(context.DeadlineExceeded, err) {
		slog.Error("failed to listen", "err", err)
		os.Exit(1)
	}
}
