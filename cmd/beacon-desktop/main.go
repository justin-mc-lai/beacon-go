package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"beacon-go/internal/ui"
)

// App is the desktop application container.
type App struct {
	server *ui.Server
	port   int
}

func main() {
	port := 4178
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := ui.NewServer(addr)

	fmt.Printf("🚀 Starting Beacon Desktop Workspace on http://%s\n", addr)

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(fmt.Sprintf("http://%s", addr))
	}()

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "server exited: %v\n", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
