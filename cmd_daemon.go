package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rokde/nudged/internal/config"
	"github.com/rokde/nudged/internal/daemon"
	"github.com/rokde/nudged/internal/proto"
)

func init() {
	register("daemon", func(args []string) int {
		socket, err := proto.SocketPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged daemon:", err)
			return 1
		}
		srv, err := daemon.Listen(socket, daemon.NewRegistry())
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged daemon:", err)
			return 1
		}
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintln(os.Stderr, "nudged daemon:", err)
		}
		if cfg.TimeoutMinutes > 0 {
			srv.HoldTimeout = time.Duration(cfg.TimeoutMinutes) * time.Minute
		}
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		go func() { <-sig; srv.Close() }()
		if err := srv.Serve(); err != nil {
			srv.Close()
			fmt.Fprintln(os.Stderr, "nudged daemon:", err)
			return 1
		}
		return 0
	})
}
