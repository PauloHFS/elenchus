package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("DefaultValues", func(t *testing.T) {
		os.Clearenv()
		cfg, err := Load()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.Port != "8080" {
			t.Errorf("expected port 8080, got %s", cfg.Port)
		}
	})

	t.Run("ProductionValidation", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("APP_ENV", "prod")
		_, err := Load()
		if err == nil {
			t.Error("expected error when SMTP_PASS is missing in production")
		}
	})

	t.Run("CustomValues", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("PORT", "9000")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if cfg.Port != "9000" {
			t.Errorf("expected port 9000, got %s", cfg.Port)
		}
	})
}
