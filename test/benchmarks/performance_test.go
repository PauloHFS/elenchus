package benchmarks

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/view"
	"github.com/PauloHFS/goth/internal/view/pages"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDB(b *testing.B) (*sql.DB, *db.Queries) {
	// Usamos um arquivo temporário para simular performance real de disco (SSD)
	// Em vez de :memory:, para testar o modo WAL de verdade
	dbFile := fmt.Sprintf("test_perf_%d.db", b.N)
	dbConn, err := sql.Open("sqlite3", dbFile+"?_journal_mode=WAL")
	if err != nil {
		b.Fatal(err)
	}

	// Executar schema básico (simplificado para o teste)
	_, _ = dbConn.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, tenant_id TEXT, email TEXT, password_hash TEXT, role_id TEXT, is_verified BOOLEAN, avatar_url TEXT, created_at DATETIME)")

	queries := db.New(dbConn)

	// Inserir 100 usuários para o teste de paginação
	for i := 0; i < 100; i++ {
		_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
			TenantID:     "default",
			Email:        fmt.Sprintf("user%d@test.com", i),
			PasswordHash: "hash",
			RoleID:       "user",
		})
	}

	b.Cleanup(func() {
		dbConn.Close()
		os.Remove(dbFile)
		os.Remove(dbFile + "-shm")
		os.Remove(dbFile + "-wal")
	})

	return dbConn, queries
}

func BenchmarkDashboardRendering(b *testing.B) {
	_, queries := setupTestDB(b)
	user := db.User{ID: 1, Email: "admin@test.com"}
	ctx := context.WithValue(context.Background(), contextkeys.UserContextKey, user)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simular lógica do handler
		page := 1
		perPage := 10
		offset := (page - 1) * perPage

		users, _ := queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
			TenantID: "default",
			Limit:    int64(perPage),
			Offset:   int64(offset),
		})

		totalUsers, _ := queries.CountUsers(ctx, "default")
		pag := view.NewPagination(page, int(totalUsers), perPage)

		// Renderizar Templ para o Recorder (mede CPU e Memória de renderização)
		w := httptest.NewRecorder()
		component := pages.Dashboard(user, users, pag)
		_ = component.Render(ctx, w)
	}
}

func BenchmarkConcurrentWebhookIngestion(b *testing.B) {
	dbConn, queries := setupTestDB(b)

	// Criar tabelas com definições completas para evitar erros de SCAN
	_, _ = dbConn.Exec(`CREATE TABLE webhooks (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		source TEXT NOT NULL, 
		external_id TEXT, 
		payload JSON NOT NULL, 
		headers JSON NOT NULL, 
		status TEXT NOT NULL DEFAULT 'pending', 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	_, _ = dbConn.Exec(`CREATE TABLE jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		tenant_id TEXT, 
		type TEXT NOT NULL, 
		payload JSON NOT NULL, 
		status TEXT NOT NULL DEFAULT 'pending', 
		idempotency_key TEXT, 
		attempt_count INTEGER DEFAULT 0, 
		max_attempts INTEGER DEFAULT 3, 
		last_error TEXT, 
		run_at DATETIME DEFAULT CURRENT_TIMESTAMP, 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, 
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			// Simular o GOTH Ingestion flow: Persistir Webhook + Criar Job
			_, err := queries.CreateWebhook(ctx, db.CreateWebhookParams{
				Source:     "stripe",
				ExternalID: sql.NullString{String: "evt_test", Valid: true},
				Payload:    []byte(`{"id": "evt_test"}`),
				Headers:    []byte(`{}`),
			})
			if err != nil {
				b.Error(err)
			}

			_, err = queries.CreateJob(ctx, db.CreateJobParams{
				TenantID: sql.NullString{String: "1", Valid: true},
				Type:     "process_webhook",
				Payload:  []byte(`{"webhook_id": 1}`),
			})
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkFTS5Search(b *testing.B) {
	dbConn, _ := setupTestDB(b)

	// Setup FTS5 schema e dados
	_, _ = dbConn.Exec(`CREATE TABLE posts (id INTEGER PRIMARY KEY, tenant_id TEXT, user_id INTEGER, title TEXT, content TEXT)`)
	_, _ = dbConn.Exec(`CREATE VIRTUAL TABLE posts_idx USING fts5(title, content, content='posts', content_rowid='id')`)

	// Inserir 1000 posts para um volume de busca razoável
	for i := 0; i < 1000; i++ {
		_, _ = dbConn.Exec(`INSERT INTO posts (tenant_id, user_id, title, content) VALUES ('default', 1, ?, ?)`,
			fmt.Sprintf("Post Title %d", i),
			fmt.Sprintf("This is the content of post %d. It contains some keywords like GOTH and SQLite.", i))
		_, _ = dbConn.Exec(`INSERT INTO posts_idx (rowid, title, content) VALUES (?, ?, ?)`,
			i+1, fmt.Sprintf("Post Title %d", i), fmt.Sprintf("This is the content of post %d. It contains some keywords like GOTH and SQLite.", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := dbConn.Query(`SELECT title FROM posts_idx WHERE posts_idx MATCH 'GOTH AND SQLite' LIMIT 20`)
		if err != nil {
			b.Fatal(err)
		}
		rows.Close()
	}
}

func BenchmarkPasswordHashing(b *testing.B) {
	password := "super-secret-password-123"

	b.Run("Bcrypt-Default", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		}
	})
}

func BenchmarkSQLiteReadWriteStress(b *testing.B) {
	dbConn, queries := setupTestDB(b)

	// Setup adicional para simular carga real
	_, _ = dbConn.Exec(`CREATE TABLE webhooks (id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT, external_id TEXT, payload JSON, headers JSON, status TEXT, created_at DATETIME)`)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()

		for pb.Next() {
			// Mix de 80% leitura e 20% escrita
			if b.N%5 == 0 {
				// Escrita
				_, _ = queries.CreateWebhook(ctx, db.CreateWebhookParams{
					Source:  "stress-test",
					Payload: []byte(`{}`),
					Headers: []byte(`{}`),
				})
			} else {
				// Leitura (Dashboard)
				_, _ = queries.ListUsersPaginated(ctx, db.ListUsersPaginatedParams{
					TenantID: "default",
					Limit:    10,
					Offset:   0,
				})
			}
		}
	})
}
