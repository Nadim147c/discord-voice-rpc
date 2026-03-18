package main

import (
	"context"
	"log/slog"
	"os"
)

func main() {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})
	slog.SetDefault(slog.New(handler))
	client := NewClient()
	client.Listen(context.Background())
}
