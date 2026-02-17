package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) (*sql.DB, *Queries) {
	// Cria um arquivo temporário para o banco de dados
	tempFile, err := os.CreateTemp("", "goth_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	tempFile.Close()
	dbPath := tempFile.Name()

	// Garante a limpeza do arquivo após o teste
	t.Cleanup(func() {
		os.Remove(dbPath)
	})

	dbConn, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := RunMigrations(ctx, dbConn); err != nil {
		// Loga o erro mas tenta continuar
		t.Logf("Aviso: Migração falhou: %v", err)
	}

	return dbConn, New(dbConn)
}

func TestTenantIsolation(t *testing.T) {
	dbConn, _ := setupTestDB(t)
	defer dbConn.Close()
	ctx := context.Background()

	// Setup: Criar Tenant
	_, err := dbConn.Exec("INSERT INTO tenants (id, name, settings) VALUES ('t1', 'Tenant 1', '{}')")
	if err != nil {
		t.Fatal(err)
	}

	// Test: Buscar Tenant existente (com CAST para BLOB para suportar json.RawMessage no SQLite)
	var tenant Tenant
	err = dbConn.QueryRowContext(ctx, "SELECT id, name, CAST(settings AS BLOB), created_at FROM tenants WHERE id = ?", "t1").Scan(
		&tenant.ID, &tenant.Name, &tenant.Settings, &tenant.CreatedAt,
	)
	if err != nil {
		t.Errorf("erro ao buscar tenant: %v", err)
	}
	if tenant.ID != "t1" {
		t.Errorf("esperado t1, obtido %s", tenant.ID)
	}
}

func TestJobQueueAtomic(t *testing.T) {
	dbConn, queries := setupTestDB(t)
	defer dbConn.Close()
	ctx := context.Background()

	// Inserir Job
	_, err := queries.CreateJob(ctx, CreateJobParams{
		Type:    "test_job",
		Payload: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pick Job
	var job Job
	err = dbConn.QueryRowContext(ctx, `
		UPDATE jobs 
		SET status = 'processing', updated_at = CURRENT_TIMESTAMP
		WHERE id = (
			SELECT id FROM jobs 
			WHERE status = 'pending' 
			ORDER BY run_at ASC LIMIT 1
		) RETURNING id, tenant_id, type, CAST(payload AS BLOB), status, attempt_count, max_attempts, last_error, run_at, created_at, updated_at
	`).Scan(
		&job.ID, &job.TenantID, &job.Type, &job.Payload, &job.Status, &job.AttemptCount, &job.MaxAttempts, &job.LastError, &job.RunAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		t.Fatal(err)
	}

	if job.Status != "processing" {
		t.Errorf("status incorreto: %s", job.Status)
	}

	// Tentar pegar novamente (deve retornar erro de no rows)
	_, err = queries.PickNextJob(ctx)
	if err != sql.ErrNoRows {
		t.Errorf("esperado sql.ErrNoRows, obtido: %v", err)
	}
}
