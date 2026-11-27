package cli

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"

	"github.com/spf13/cobra"
)

func newTreeServeCmd() *cobra.Command {
	var project, version string
	cmd := &cobra.Command{
		Use: "serve", Short: "Open a loopback tree workspace with AC/test-case binding",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, _, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				return err
			}
			defer listener.Close()
			secret := make([]byte, 32)
			if _, err := rand.Read(secret); err != nil {
				return err
			}
			origin := "http://" + listener.Addr().String()
			server := &http.Server{Handler: newTreeBindingHandler(st, dir, origin, hex.EncodeToString(secret)),
				ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: time.Minute}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			done := make(chan struct{})
			go func() {
				defer close(done)
				<-ctx.Done()
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdown); err != nil {
					_ = server.Close()
				}
			}()
			fmt.Fprintln(cmd.OutOrStdout(), "tree workspace: "+origin+"/ (Ctrl+C to stop)")
			err = server.Serve(listener)
			stop()
			<-done
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", ".", "project root")
	cmd.Flags().StringVar(&version, "version", "", "docs version (default current)")
	return cmd
}

func newTreeBindingHandler(st *store.Store, dir, origin, token string) http.Handler {
	var mu sync.Mutex
	address, _ := url.Parse(origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		if address == nil || r.Host != address.Host || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin" && r.Header.Get("Sec-Fetch-Site") != "none") {
			http.Error(w, "tree_binding_origin_forbidden", http.StatusForbidden)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/" && r.Method == http.MethodGet:
			idx, err := intenttree.ReadIndex(dir)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			project := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir))))
			page, err := renderCurrentTree(dir, project, idx, intenttree.RenderContext{
				RequirementVersion: idx.Version, BindingURL: "/binding", BindingToken: token,
			})
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		case r.URL.Path == "/binding" && r.Method == http.MethodPost:
			if r.Header.Get("Origin") != origin {
				http.Error(w, "tree_binding_origin_forbidden", 403)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 8192)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "tree_binding_invalid_form", 400)
				return
			}
			keys := []string{"node", "base_fingerprint", "test_case_id", "ac_id", "csrf_token"}
			if len(r.Form) != len(keys) || len(r.URL.RawQuery) != 0 {
				http.Error(w, "tree_binding_invalid_form", 400)
				return
			}
			for _, key := range keys {
				values := r.PostForm[key]
				if len(values) != 1 || strings.TrimSpace(values[0]) == "" || strings.TrimSpace(values[0]) != values[0] {
					http.Error(w, "tree_binding_invalid_form", 400)
					return
				}
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf_token")), []byte(token)) != 1 {
				http.Error(w, "tree_binding_token_forbidden", 403)
				return
			}
			node, status, err := updateTreeBinding(st, dir, r.PostForm.Get("node"), r.PostForm.Get("base_fingerprint"), r.PostForm.Get("test_case_id"), r.PostForm.Get("ac_id"))
			if err != nil {
				http.Error(w, err.Error(), status)
				return
			}
			if r.Header.Get("Accept") != "application/json" {
				http.Redirect(w, r, "/#"+url.PathEscape(node.ID), http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(node)
		default:
			http.Error(w, "tree_binding_route_not_found", 404)
		}
	})
}

func updateTreeBinding(st *store.Store, dir, nodeID, base, caseID, acID string) (intenttree.Node, int, error) {
	updated, err := intenttree.UpdateNode(st, dir, nodeID, intenttree.NodePatch{
		ExpectedFingerprint: base, TestCaseBinding: &intenttree.TestCaseBinding{ID: caseID, ACID: acID},
	})
	if err != nil {
		if reason, ok := core.ReasonCodeOf(err); ok {
			if reason == intenttree.ReasonChangeBaseMismatch || reason == intenttree.ReasonNodeNotDraft {
				return intenttree.Node{}, 409, err
			}
			if reason == intenttree.ReasonNodeNotFound {
				return intenttree.Node{}, 404, err
			}
		}
		return intenttree.Node{}, 422, err
	}
	return updated, 200, nil
}
