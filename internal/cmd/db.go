package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/logging"
)

func initDB() (*sql.DB, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	// Aplicando os mesmos pragmas de performance e resiliência
	dsn := cfg.DatabaseURL
	if strings.Contains(dsn, "?") {
		dsn += "&_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	} else {
		dsn += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	dbConn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return dbConn, nil
}

func RunSeed() {
	dbConn, err := initDB()
	if err != nil {
		panic(err)
	}
	defer dbConn.Close()

	logging.Init()
	logger := logging.Get()

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations during seed", "error", err)
		return
	}
	if err := db.Seed(context.Background(), dbConn); err != nil {
		logger.Error("failed to seed database", "error", err)
		return
	}
	logger.Info("database seeded successfully")
}

func RunMigrate() {
	dbConn, err := initDB()
	if err != nil {
		panic(err)
	}
	defer dbConn.Close()

	logging.Init()
	logger := logging.Get()

	if err := db.RunMigrations(context.Background(), dbConn); err != nil {
		logger.Error("failed to run migrations", "error", err)
		return
	}
	logger.Info("migrations executed successfully")
}
