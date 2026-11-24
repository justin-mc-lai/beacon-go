//go:build ignore

// Prototype tool (ticket #17): capture the CURRENT sea/lake decomposition
// engine's raw output for the shared badminton example. Not part of the
// kernel; excluded from ./... by the ignore build tag and the underscore
// directory.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"beacon-go/internal/core"
)

func main() {
	utterance := "构建一个羽毛球运动社区小程序，有赛事功能、社区功能"
	if len(os.Args) > 1 {
		utterance = os.Args[1]
	}
	d := core.DecomposeIntent(utterance, nil, "", "2026-11-24T00:00:00Z")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(d); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
