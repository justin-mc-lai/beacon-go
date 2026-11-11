// `goal serve` (kernel-readonly-http-api): an opt-in, loopback-only HTTP
// projection of the runtime store. Without --http the command prints a
// disabled notice and exits 0; with --http it serves five GET endpoints
// until SIGINT/SIGTERM and then releases the listener. The store is
// opened read-only — this command has no write surface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/httpapi"
)

// ReasonRuntimeStoreNotFound is the CLI code for `goal serve --http`
// before any runtime DB exists.
const ReasonRuntimeStoreNotFound = "runtime_store_not_found"

const defaultServeAddr = "127.0.0.1:8787"

func newGoalServeCmd() *cobra.Command {
	var enableHTTP bool
	var addr string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Read-only HTTP/JSON projection of the runtime spine (default off, loopback only)",
		Long: "Serves GET /health, /spine/runs, /spine/gates?feature=, /runs/{id} and " +
			"/features/{slug} over the runtime store. The server is disabled unless " +
			"--http is passed, binds only 127.0.0.1 or ::1, and opens the store " +
			"read-only: it never advances a run, writes a gate or releases anything.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !enableHTTP {
				fmt.Fprintln(cmd.OutOrStdout(), "beacon goal serve: HTTP API disabled (pass --http to listen on "+defaultServeAddr+")")
				return nil
			}
			if err := httpapi.ValidateLoopbackAddr(addr); err != nil {
				return fail("non_loopback_addr", "goal serve", err.Error())
			}
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonRuntimeStoreNotFound, "goal serve", "no runtime store yet; create a goal run first")
			}
			defer st.Close()

			srv := &http.Server{
				Handler:           httpapi.New(st, Version).Handler(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fail("serve_listen_failed", "goal serve", err.Error())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "beacon goal serve: listening on http://%s (read-only; Ctrl-C to stop)\n", ln.Addr().String())

			serveErr := make(chan error, 1)
			go func() { serveErr <- srv.Serve(ln) }()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			select {
			case err := <-serveErr:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return fail("serve_failed", "goal serve", err.Error())
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
					_ = srv.Close()
				}
				<-serveErr
				return nil
			}
		},
	}
	c.Flags().BoolVar(&enableHTTP, "http", false, "enable the read-only HTTP API (default off)")
	c.Flags().StringVar(&addr, "addr", defaultServeAddr, "listen address; host must be 127.0.0.1 or ::1")
	return c
}
