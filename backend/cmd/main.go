package main

import (
	"context"
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
	address := os.Getenv("WEDDINGHUB_ADDR")
	if address == "" {
		address = ":8080"
	}

	// PostgreSQL is the only supported store. There is no in-memory fallback, so a
	// missing or unreachable database is a fatal startup error rather than a
	// silently degraded demo.
	dsn := strings.TrimSpace(os.Getenv("WEDDINGHUB_DATABASE_URL"))
	if dsn == "" {
		log.Fatal("WEDDINGHUB_DATABASE_URL is required: WeddingHub stores its data in PostgreSQL and no longer runs on an in-memory repository")
	}
	connectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := repository.NewPostgresRepository(connectCtx, dsn)
	if err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer repo.Close()

	apiHandler := api.New(repo)

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
