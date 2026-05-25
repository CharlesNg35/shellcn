// Command server is the ShellCN gateway entrypoint.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/charlesng/shellcn/web"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		dev         bool
		addr        string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&dev, "dev", false, "dev mode: serve the API only; Vite serves the UI")
	flag.StringVar(&addr, "addr", ":8080", "address to listen on")
	flag.Parse()

	if showVersion {
		fmt.Printf("shellcn %s\n", version)
		return
	}

	if err := run(addr, dev); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run(addr string, dev bool) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	if dev {
		slog.Info("dev mode: API only, Vite serves the UI", "addr", addr)
	} else {
		dist, err := web.FS()
		if err != nil {
			return fmt.Errorf("load embedded frontend: %w", err)
		}
		mux.Handle("/", http.FileServerFS(dist))
		slog.Info("starting", "addr", addr, "version", version)
	}

	if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
