package cmd

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/PauloHFS/elenchus/docs"
	"github.com/PauloHFS/elenchus/internal/config"
	"github.com/PauloHFS/elenchus/internal/db"
	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/PauloHFS/elenchus/internal/middleware"
	"github.com/PauloHFS/elenchus/internal/sse"
	"github.com/PauloHFS/elenchus/internal/web"
	"github.com/PauloHFS/elenchus/internal/webhook"
	"github.com/PauloHFS/elenchus/internal/worker"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggo/http-swagger"
)

// @title GOTH Stack API
// @version 1.0
// @description API do boilerplate GOTH Stack (Go, Templ, HTMX).
// @host localhost:8080
// @BasePath /
func RunServer(assetsFS embed.FS) {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	logging.Init()
	logger := logging.Get()

	// 1. DB (Hardening para Produção)
	dsn := cfg.DatabaseURL
	if strings.Contains(dsn, "?") {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	} else {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	dbConn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		panic(err)
	}
	defer dbConn.Close()

	// 1.1 Garantir diretórios de storage
	if err := os.MkdirAll("storage/avatars", 0755); err != nil {
		logger.Error("failed to create storage directories", "error", err)
		panic(err)
	}

	queries := db.New(dbConn)

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations", "error", err)
		panic(err)
	}

	sessionManager := scs.New()
	sessionManager.Store = sqlite3store.New(dbConn)

	// Create SSE Broker
	broker := sse.NewBroker()

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := worker.New(cfg, dbConn, queries, logger, broker)
	if err := w.RescueZombies(workerCtx); err != nil {
		logger.Error("zombie hunter failed", "error", err)
	}
	go w.Start(workerCtx)

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))
	mux.Handle("GET /storage/", http.StripPrefix("/storage/", http.FileServer(http.Dir("storage"))))
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	mux.Handle("POST /webhooks/{source}", webhook.NewHandler(queries))

	mux.HandleFunc("GET "+web.Health, func(w http.ResponseWriter, r *http.Request) {
		// 1. Ping DB
		if err := dbConn.PingContext(r.Context()); err != nil {
			logger.Error("health check failed: db unreachable", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// 2. Check Jobs Health
		var failedJobs int
		_ = dbConn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM jobs WHERE status = 'failed'").Scan(&failedJobs)

		var pendingJobs int
		_ = dbConn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM jobs WHERE status = 'pending'").Scan(&pendingJobs)

		if failedJobs > 50 || pendingJobs > 1000 {
			logger.Warn("health check warning: job queue issues", "failed", failedJobs, "pending", pendingJobs)
		}

		// 3. Disk Space Check
		var stat syscall.Statfs_t
		wd, _ := os.Getwd()
		err := syscall.Statfs(wd, &stat)
		if err == nil {
			freeSpace := stat.Bavail * uint64(stat.Bsize)
			if freeSpace < 100*1024*1024 { // Menos de 100MB
				logger.Error("health check failed: low disk space", "free_bytes", freeSpace)
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, "Low disk space")
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Registrar handlers de negócio
	web.RegisterRoutes(mux, web.HandlerDeps{
		DB:             dbConn,
		Queries:        queries,
		SessionManager: sessionManager,
		Logger:         logger,
		Config:         cfg,
		SSEBroker:      broker,
	})

	// Ordem dos middlewares (de fora para dentro):
	// Recovery -> Logger -> RateLimit -> SecurityHeaders -> Locale -> Session -> CSRF
	// Logger vem cedo para capturar TUDO, incluindo falhas CSRF e rate limit
	handler := middleware.Recovery(
		middleware.Logger(
			middleware.RateLimit(
				middleware.SecurityHeaders(cfg.Env == "production")(
					middleware.Locale(
						sessionManager.LoadAndSave(
							middleware.CSRFWithContext(mux),
						),
					),
				),
			),
		),
	)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("server stopping")

	cancelWorker()
	w.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server exited properly")
}
