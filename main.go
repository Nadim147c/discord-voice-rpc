package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
)

var (
	buildType   = "debug"
	noDebugFlag bool
	debug       bool
)

func init() {
	flag.BoolVar(&noDebugFlag, "no-debug", false, "disable debug logging")
}

func main() {
	flag.Parse()
	debug = buildType == "debug" && !noDebugFlag

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
	var client *Client
	var wg sync.WaitGroup

	wg.Go(func() {
		r := NewContextReader(ctx, os.Stdin)
		scanner := bufio.NewScanner(r)

		for scanner.Scan() {
			cmd := scanner.Text()
			s := strings.SplitN(cmd, ":", 3)
			if len(s) != 3 {
				slog.Warn("invalid command format", "command", cmd)
				continue
			}

			req, id, arg := s[0], s[1], s[2]

			switch Request(req) {
			case ReqMute:
				v, err := strconv.ParseBool(arg)
				if err != nil {
					slog.Error("failed to parse mute value", "arg", arg, "err", err)
					continue
				}

				if err := client.setUserMute(id, v); err != nil {
					slog.Error("failed to set user mute", "id", id, "err", err)
				}

			case ReqVolume:
				v, err := strconv.Atoi(arg)
				if err != nil {
					slog.Error("failed to parse volume value", "arg", arg, "err", err)
					continue
				}

				if err := client.setUserVolume(id, uint(v)); err != nil {
					slog.Error("failed to set user volume", "id", id, "err", err)
				}

			default:
				slog.Warn("unknown request type", "request", req)
			}
		}

		if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("stdin scanner error", "err", err)
		}
	})

	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				client = NewClient()
				err := client.Listen(ctx)
				if err != nil && !errors.Is(err, ErrIpcNotFound) && !isDone(ctx) {
					slog.Error("failed to listen", "err", err)
				}
			}
		}
	})

	wg.Wait()
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
