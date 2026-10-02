package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

var (
	buildType   = "debug"
	noDebugFlag bool
)

var debug = buildType == "debug" && !noDebugFlag

func init() {
	flag.BoolVar(&noDebugFlag, "no-debug", false, "disable debug logging")
}

func main() {
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM) // only works on linux
	defer cancel()

	var level log.Level // zero value is info
	if debug {
		level = log.DebugLevel
	}

	handler := log.NewWithOptions(os.Stderr, log.Options{
		TimeFunction: nil,
		Level:        level,
	})
	slog.SetDefault(slog.New(handler))

	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			client := NewClient()
			err := client.Listen(ctx)
			if err != nil && !isDone(ctx) {
				slog.Error("failed to listen", "err", err)
				os.Exit(1)
			}
		}
	}
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
