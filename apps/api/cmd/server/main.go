// Command server runs the CompatLab REST API.
//
// Configuration (environment variables):
//
//	APP_PORT                 listen port (default 8080)
//	STORE                    "memory" (default, data is lost on exit) or "postgres"
//	DATABASE_URL             PostgreSQL connection string (required for STORE=postgres)
//	JWT_SECRET               token signing secret, at least 32 bytes (required)
//	ALLOWED_TARGET_HOSTS     comma-separated hosts the test runner may contact (default: none)
//	MAX_BODY_BYTES           maximum request body size (default 5242880)
//	RATE_LIMIT_PER_MINUTE    API requests per IP per minute (default 300; 0 disables)
//	AUTH_RATE_LIMIT_PER_MINUTE  register/login requests per IP per minute (default 20)
//	CORS_ALLOWED_ORIGIN      single browser origin allowed by CORS (default: none)
//	TRUSTED_PROXIES          comma-separated proxy IPs/CIDRs whose X-Forwarded-For is trusted
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/thanhmaitran/compatlab/apps/api/routes"
	"github.com/thanhmaitran/compatlab/database"
	"github.com/thanhmaitran/compatlab/internal/auth"
	"github.com/thanhmaitran/compatlab/internal/persistence"
	"github.com/thanhmaitran/compatlab/internal/service"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s must be an integer: %v", key, err)
	}
	return n
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// healthcheck probes the local /health endpoint; used by container health checks
// (the runtime image has no shell or curl).
func healthcheck() int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + env("APP_PORT", "8080") + "/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		os.Exit(healthcheck())
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokens, err := auth.NewTokenIssuer(os.Getenv("JWT_SECRET"), 12*time.Hour)
	if err != nil {
		log.Fatalf("JWT_SECRET: %v", err)
	}

	var store persistence.Store
	switch env("STORE", "memory") {
	case "postgres":
		pg, err := persistence.NewPostgres(ctx, os.Getenv("DATABASE_URL"))
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		if err := pg.Migrate(ctx, database.Migrations, "migrations"); err != nil {
			log.Fatalf("migrations: %v", err)
		}
		store = pg
	case "memory":
		log.Println("WARNING: using the in-memory store; data is lost when the server stops")
		store = persistence.NewMemory()
	default:
		log.Fatal("STORE must be memory or postgres")
	}
	defer store.Close()

	svc := service.New(store, tokens, split(os.Getenv("ALLOWED_TARGET_HOSTS")))
	router := routes.New(routes.Config{
		MaxBodyBytes:   int64(envInt("MAX_BODY_BYTES", 5<<20)),
		RateLimit:      envInt("RATE_LIMIT_PER_MINUTE", 300),
		AuthRateLimit:  envInt("AUTH_RATE_LIMIT_PER_MINUTE", 20),
		CORSOrigin:     os.Getenv("CORS_ALLOWED_ORIGIN"),
		TrustedProxies: split(os.Getenv("TRUSTED_PROXIES")),
	}, svc)

	srv := &http.Server{
		Addr:              ":" + env("APP_PORT", "8080"),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      3 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("CompatLab API listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
