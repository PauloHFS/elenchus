package db

import (
	"context"
	"database/sql"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func Seed(ctx context.Context, dbConn *sql.DB) error {
	queries := New(dbConn)

	// 1. Criar Tenant Padrão
	_, err := dbConn.ExecContext(ctx, "INSERT OR IGNORE INTO tenants (id, name) VALUES ('default', 'Default Tenant')")
	if err != nil {
		return fmt.Errorf("failed to seed tenant: %w", err)
	}

	// 2. Criar Role Admin
	_, err = dbConn.ExecContext(ctx, `INSERT OR IGNORE INTO roles (id, permissions) VALUES ('admin', '["*"]')`)
	if err != nil {
		return fmt.Errorf("failed to seed role: %w", err)
	}

	// 3. Criar Usuário Admin (admin@admin.com / admin123)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	_, err = queries.CreateUser(ctx, CreateUserParams{
		TenantID:     "default",
		Email:        "admin@admin.com",
		PasswordHash: string(hash),
		RoleID:       "admin",
	})
	if err != nil {
		// Se já existir, ignoramos
		return nil
	}

	fmt.Println("✅ Database seeded successfully: admin@admin.com / admin123")
	return nil
}
