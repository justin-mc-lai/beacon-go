// Command beacon is the Go-track Beacon CLI entrypoint
// (counterpart of beacon/main.py + beacon/__main__.py in the v1 Python track).
package main

import (
	"fmt"
	"os"

	"beacon-go/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
