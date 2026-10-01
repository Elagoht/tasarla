// Command kanban serves the kanban application.
//
// "collage dev" builds this program and runs it with COLLAGE_DEV=1, the
// variables in .env.development, and HOST and PORT set; this file honours all
// three. Every page is private, so "collage export" has nothing to write and
// this program does not implement -collage-build.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"strconv"
	"time"

	"kanban/internal/auth"
	"kanban/internal/config"
	"kanban/internal/db"
	"kanban/internal/files"
	"kanban/internal/store"
	"kanban/internal/web"
)

// The binary carries its templates, static files and catalogs.
//
//go:embed all:templates all:static locales
var embedded embed.FS

func main() {
	if err := run(); err != nil {
		log.Fatalf("kanban: %v", err)
	}
}

func run() error {
	devMode := os.Getenv("COLLAGE_DEV") == "1"
	logger := slog.Default()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("configuration:\n%w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	client, err := auth.NewClient(ctx, auth.ClientConfig{
		Issuer: cfg.OIDC.Issuer, ClientID: cfg.OIDC.ClientID, ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL: cfg.OIDC.RedirectURL, Scopes: cfg.OIDC.Scopes,
	})
	if err != nil {
		return err
	}

	assets, err := appFiles(devMode)
	if err != nil {
		return err
	}
	attachments, err := files.Open(cfg.AttachmentsDir)
	if err != nil {
		return err
	}
	app, err := web.New(web.Deps{
		Files: assets, Attachments: attachments, DevMode: devMode,
		Host: envString("HOST", "localhost"), Port: envInt("PORT", 6060),
		Config: cfg, Store: store.New(pool), OIDC: client, Logger: logger,
	})
	if err != nil {
		return err
	}
	return app.ListenAndServe()
}

// appFiles is the embedded copy, or in development the working directory, so
// an edited template or catalog shows on the next request.
func appFiles(devMode bool) (fs.FS, error) {
	if devMode {
		if root, err := os.OpenRoot("."); err == nil {
			return root.FS(), nil
		}
	}
	return embedded, nil
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}
