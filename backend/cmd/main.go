package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
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

	// Frontend source directory for local runs; deployments point this at dist.
	staticDir := os.Getenv("WEDDINGHUB_STATIC_DIR")
	if staticDir == "" {
		if _, err := os.Stat("../frontend/pages"); err == nil {
			staticDir = "../frontend"
		} else if _, err := os.Stat("frontend/pages"); err == nil {
			staticDir = "frontend"
		}
	}

	var rootHandler http.Handler = apiHandler
	if staticDir != "" {
		absPath, _ := filepath.Abs(staticDir)
		log.Printf("Serving static frontend from %s", absPath)
		frontendHandler, err := newFrontendHandler(staticDir)
		if err != nil {
			log.Fatalf("register frontend pages: %v", err)
		}
		rootHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
				apiHandler.ServeHTTP(w, r)
				return
			}
			frontendHandler.ServeHTTP(w, r)
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

// newFrontendHandler registers every HTML document at a clean extensionless
// route. Legacy .html URLs permanently redirect to that route. Non-page
// requests fall through to the file server for assets such as CSS and JS.
func newFrontendHandler(staticDir string) (http.Handler, error) {
	pages := http.NewServeMux()
	files := http.FileServer(http.Dir(staticDir))
	assets := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		file, err := http.Dir(staticDir).Open(filePath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, err := file.Stat()
		file.Close()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
	err := filepath.WalkDir(staticDir, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".html" {
			return nil
		}
		relativePath, err := filepath.Rel(staticDir, filePath)
		if err != nil {
			return err
		}
		fileRoute := "/" + filepath.ToSlash(relativePath)
		routeSource := filepath.ToSlash(relativePath)
		fromPagesDir := strings.HasPrefix(routeSource, "pages/")
		if fromPagesDir {
			routeSource = strings.TrimPrefix(routeSource, "pages/")
		}
		pageRoute := "/" + strings.TrimSuffix(routeSource, filepath.Ext(routeSource))
		if filepath.Base(filePath) == "index.html" {
			directory := filepath.ToSlash(filepath.Dir(relativePath))
			if directory == "." {
				pageRoute = "/"
			} else {
				pageRoute = "/" + directory
			}
		}
		registerPage := func(pattern, target, baseHref string) {
			pages.HandleFunc("GET "+pattern, func(w http.ResponseWriter, r *http.Request) {
				servePage(w, r, target, baseHref)
			})
		}
		registerRedirect := func(pattern, target string) {
			pages.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				location := target
				if r.URL.RawQuery != "" {
					location += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			})
		}
		pagePattern := pageRoute
		if pagePattern == "/" {
			pagePattern = "/{$}"
		}
		baseHref := ""
		if fromPagesDir {
			baseHref = "/pages/"
		}
		registerPage(pagePattern, filePath, baseHref)
		registerRedirect(fileRoute, pageRoute)
		if fromPagesDir {
			legacyExtensionlessRoute := "/" + strings.TrimSuffix(filepath.ToSlash(relativePath), filepath.Ext(relativePath))
			registerRedirect(legacyExtensionlessRoute, pageRoute)
		}
		if fileRoute != "/index.html" && filepath.Base(filePath) == "index.html" {
			registerRedirect(pageRoute+"/{$}", pageRoute)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	pages.Handle("/", assets)
	return pages, nil
}

func servePage(w http.ResponseWriter, r *http.Request, filePath, baseHref string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if baseHref != "" {
		markup := string(content)
		if headStart := strings.Index(strings.ToLower(markup), "<head"); headStart >= 0 {
			if headEnd := strings.IndexByte(markup[headStart:], '>'); headEnd >= 0 {
				insertionPoint := headStart + headEnd + 1
				markup = markup[:insertionPoint] + `<base href="` + baseHref + `">` + markup[insertionPoint:]
			}
		}
		content = []byte(markup)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, filepath.Base(filePath), time.Time{}, bytes.NewReader(content))
}
