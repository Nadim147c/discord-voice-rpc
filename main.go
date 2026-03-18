package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM) // only works on linux
	defer cancel()

	handler := log.NewWithOptions(os.Stderr, log.Options{TimeFunction: nil})
	slog.SetDefault(slog.New(handler))
	client := NewClient()
	client.Listen(ctx)
}
