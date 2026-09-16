package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"weddinghub/api"
	"weddinghub/repository"
)

func main() {
	// Render and similar PaaS providers inject PORT and require the server
	// to bind 0.0.0.0 on that port; WEDDINGHUB_ADDR still wins for manual runs.
	address := os.Getenv("WEDDINGHUB_ADDR")
	if address == "" {
		if port := os.Getenv("PORT"); port != "" {
			address = "0.0.0.0:" + port
		} else {
			address = ":8080"
		}
	}

	apiHandler := api.New(repository.NewMemoryRepository())

	// Static files directory (weddinghub root directory)
	staticDir := os.Getenv("WEDDINGHUB_STATIC_DIR")
	if staticDir == "" {
		if _, err := os.Stat("../pages"); err == nil {
			staticDir = ".."
		} else if _, err := os.Stat("pages"); err == nil {
			staticDir = "."
		}
	}

	var rootHandler http.Handler = apiHandler
	if staticDir != "" {
		absPath, _ := filepath.Abs(staticDir)
		log.Printf("Serving static frontend from %s", absPath)
		fileServer := http.FileServer(http.Dir(staticDir))
		rootHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
				apiHandler.ServeHTTP(w, r)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	server := &http.Server{
		Addr:              address,
		Handler:           rootHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("WeddingHub server listening on %s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
