package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /info", infoHandler)

	log.Printf("listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
