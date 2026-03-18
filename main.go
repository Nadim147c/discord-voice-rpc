package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/charmbracelet/log"
)

func main() {
	handler := log.NewWithOptions(os.Stderr, log.Options{TimeFunction: nil})
	slog.SetDefault(slog.New(handler))
	client := NewClient()
	client.Listen(context.Background())
}
