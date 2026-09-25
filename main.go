package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	versionFile = ".version"
	secretFile  = ".build-secret"
)

type infoResponse struct {
	Version     string `json:"version"`
	BuildSecret string `json:"build_secret"`
	Hostname    string `json:"hostname"`
}

// readFileOr returns the trimmed contents of path, or fallback when the file is
// missing or unreadable.
func readFileOr(path, fallback string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	if v := strings.TrimSpace(string(b)); v != "" {
		return v
	}
	return fallback
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	resp := infoResponse{
		Version:     readFileOr(versionFile, "unknown"),
		BuildSecret: readFileOr(secretFile, "unknown"),
		Hostname:    hostname,
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

// runServer serves the /info endpoint until ctx is cancelled.
func runServer(ctx context.Context) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /info", infoHandler)
	srv := &http.Server{Addr: ":" + port, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Print("shutting down")
		// Fresh context: ctx is already cancelled, so it cannot bound the drain.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// runWorker prints a line on every tick until ctx is cancelled.
func runWorker(ctx context.Context) error {
	interval := 5 * time.Second
	if v := os.Getenv("WORKER_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid WORKER_INTERVAL %q: %w", v, err)
		}
		interval = d
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("worker started, interval %s", interval)
	for {
		select {
		case <-ticker.C:
			log.Print("Ah, ha, ha, ha, stayin' alive, stayin' alive")
		case <-ctx.Done():
			log.Print("worker stopping")
			return nil
		}
	}
}

func main() {
	// Containers get SIGTERM on stop; without this the runtime waits out its
	// grace period and then SIGKILLs.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cmd := "server"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "server":
		err = runServer(ctx)
	case "worker":
		err = runWorker(ctx)
	default:
		log.Fatalf("unknown command %q (want \"server\" or \"worker\")", cmd)
	}

	if err != nil {
		log.Fatal(err)
	}
}
