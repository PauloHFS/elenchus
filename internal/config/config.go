package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPass      string
	SMTPFrom      string
	SessionSecret string
	Env           string // "dev" or "prod"
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "./elenchus.db"),
		SMTPHost:      getEnv("SMTP_HOST", "localhost"),
		SMTPPort:      getEnv("SMTP_PORT", "1025"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      getEnv("SMTP_FROM", "noreply@elenchus.com"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		Env:           getEnv("ENV", "development"),
	}

	// Validação Estrita para Produção
	if cfg.Env == "production" {
		if cfg.SMTPPass == "" {
			return nil, fmt.Errorf("produção: SMTP_PASS é obrigatório")
		}
		if cfg.SMTPUser == "" {
			return nil, fmt.Errorf("produção: SMTP_USER é obrigatório")
		}
		if cfg.SessionSecret == "" {
			return nil, fmt.Errorf("produção: SESSION_SECRET é obrigatório")
		}
	} else {
		// No dev, se não houver secret, usamos um valor fraco apenas para não quebrar o boot
		if cfg.SessionSecret == "" {
			cfg.SessionSecret = "elenchus-dev-secret-not-safe"
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		var result int
		fmt.Sscanf(value, "%d", &result)
		if result > 0 {
			return result
		}
	}
	return fallback
}
