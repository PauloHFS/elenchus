package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/PauloHFS/goth/migrations"
)

// RunMigrations executa todos os arquivos .sql do FS embutido em ordem alfabética.
func RunMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("falha ao ler diretório de migrações: %w", err)
	}

	var filenames []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			filenames = append(filenames, e.Name())
		}
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("falha ao ler arquivo %s: %w", name, err)
		}

		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("falha ao executar migração %s: %w", name, err)
		}
	}

	return nil
}
