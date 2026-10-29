// Command beacon is the Go-track Beacon CLI entrypoint
// (counterpart of beacon/main.py + beacon/__main__.py in the v1 Python track).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"beacon-go/internal/cli"
)

func main() {
	// AC-105: a long-running command (goal driver run-loop, ralph) must be
	// interruptible without leaving the host in an unknown state. The
	// signal cancels the command context; engines that honour it stop
	// between turns and record host_interrupted.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
