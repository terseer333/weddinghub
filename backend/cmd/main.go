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

	"golang.org/x/crypto/bcrypt"

	"weddinghub/api"
	"weddinghub/config"
	"weddinghub/mailer"
	"weddinghub/repository"
)

func main() {
	// A .env file is optional and never overrides the real environment, so one set of names works
	// for a local run and for a host's configured variables.
	if envFile, err := config.LoadEnvFile(); err != nil {
		log.Fatalf("read environment file: %v", err)
	} else if envFile != "" {
		log.Printf("Loaded environment from %s", envFile)
	}

	address := config.ListenAddress()

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
	adminEmail := config.AdminEmail()
	adminHash := strings.TrimSpace(os.Getenv("WEDDINGHUB_ADMIN_PASSWORD_HASH"))
	if adminHash != "" {
		if adminEmail == "" {
			log.Fatal("WEDDINGHUB_ADMIN_EMAIL is required when WEDDINGHUB_ADMIN_PASSWORD_HASH is set")
		}
		if _, err := bcrypt.Cost([]byte(adminHash)); err != nil {
			log.Fatal("WEDDINGHUB_ADMIN_PASSWORD_HASH must be a bcrypt hash")
		}
		if err := repo.EnsurePlatformAdmin(adminEmail, adminHash, time.Now().UTC()); err != nil {
			log.Fatalf("configure platform administrator: %v", err)
		}
	}

	passwordMailer, mailerErr := mailer.FromEnv()
	if mailerErr != nil {
		log.Printf("password recovery email is disabled: %v", mailerErr)
	}
	apiHandler := api.NewWithPasswordMailer(repo, passwordMailer)

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
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/i/") || strings.HasPrefix(r.URL.Path, "/og/") || r.URL.Path == "/static/og/weddinghub-default.png" {
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
