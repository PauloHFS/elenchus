package policies

import (
	"context"
	"testing"

	"github.com/PauloHFS/elenchus/internal/db"
)

func TestCanAccessEvaluation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		user       db.User
		evaluation db.Evaluation
		expected   bool
	}{
		{
			name: "admin can access any evaluation",
			user: db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-b", UserID: 2},
			expected: true,
		},
		{
			name: "user can access own tenant evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-a", UserID: 1},
			expected: true,
		},
		{
			name: "user cannot access other tenant evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-b", UserID: 2},
			expected: false,
		},
		{
			name: "administrator can access any evaluation",
			user: db.User{ID: 1, RoleID: "administrator", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-c", UserID: 3},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanAccessEvaluation(ctx, tt.user, tt.evaluation)
			if result != tt.expected {
				t.Errorf("CanAccessEvaluation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanCreateEvaluation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		user     db.User
		tenantID string
		expected bool
	}{
		{
			name:     "unauthenticated user cannot create",
			user:     db.User{ID: 0, RoleID: "", TenantID: ""},
			tenantID: "tenant-a",
			expected: false,
		},
		{
			name:     "admin can create in any tenant",
			user:     db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			tenantID: "tenant-b",
			expected: true,
		},
		{
			name:     "user can create in own tenant",
			user:     db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			tenantID: "tenant-a",
			expected: true,
		},
		{
			name:     "user cannot create in other tenant",
			user:     db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			tenantID: "tenant-b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanCreateEvaluation(ctx, tt.user, tt.tenantID)
			if result != tt.expected {
				t.Errorf("CanCreateEvaluation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanDeleteEvaluation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		user       db.User
		evaluation db.Evaluation
		expected   bool
	}{
		{
			name: "admin can delete any evaluation",
			user: db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-b", UserID: 2},
			expected: true,
		},
		{
			name: "creator can delete own evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-a", UserID: 1},
			expected: true,
		},
		{
			name: "user cannot delete other user evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-a", UserID: 2},
			expected: false,
		},
		{
			name: "regular user cannot delete without user_id match",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-a", UserID: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanDeleteEvaluation(ctx, tt.user, tt.evaluation)
			if result != tt.expected {
				t.Errorf("CanDeleteEvaluation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckEvaluationAccess(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		user       db.User
		evaluation db.Evaluation
		action     Action
		wantErr    bool
	}{
		{
			name: "unauthenticated user gets error",
			user: db.User{ID: 0},
			action: ActionView,
			wantErr: true,
		},
		{
			name: "user can view own tenant evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{TenantID: "tenant-a", Status: "processing"},
			action: ActionView,
			wantErr: false,
		},
		{
			name: "cannot edit completed evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{TenantID: "tenant-a", Status: "completed"},
			action: ActionEdit,
			wantErr: true,
		},
		{
			name: "user cannot delete other evaluation",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{TenantID: "tenant-a", UserID: 2},
			action: ActionDelete,
			wantErr: true,
		},
		{
			name: "non-admin cannot perform audit",
			user: db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			evaluation: db.Evaluation{TenantID: "tenant-a"},
			action: ActionAudit,
			wantErr: true,
		},
		{
			name: "admin can perform audit",
			user: db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			evaluation: db.Evaluation{TenantID: "tenant-a"},
			action: ActionAudit,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckEvaluationAccess(ctx, tt.user, tt.evaluation, tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckEvaluationAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckTenantAccess(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		user     db.User
		tenantID string
		wantErr  bool
	}{
		{
			name:     "unauthenticated user gets error",
			user:     db.User{ID: 0},
			tenantID: "tenant-a",
			wantErr:  true,
		},
		{
			name:     "admin can access any tenant",
			user:     db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			tenantID: "tenant-b",
			wantErr:  false,
		},
		{
			name:     "user can access own tenant",
			user:     db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			tenantID: "tenant-a",
			wantErr:  false,
		},
		{
			name:     "user cannot access other tenant",
			user:     db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			tenantID: "tenant-b",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTenantAccess(ctx, tt.user, tt.tenantID)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckTenantAccess() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCanViewAudit(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		user       db.User
		audit      db.Audit
		evaluation db.Evaluation
		expected   bool
	}{
		{
			name:       "admin can view any audit",
			user:       db.User{ID: 1, RoleID: "admin", TenantID: "tenant-a"},
			audit:      db.Audit{ID: "audit-1"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-b"},
			expected:   true,
		},
		{
			name:       "user can view own tenant audit",
			user:       db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			audit:      db.Audit{ID: "audit-1"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-a"},
			expected:   true,
		},
		{
			name:       "user cannot view other tenant audit",
			user:       db.User{ID: 1, RoleID: "user", TenantID: "tenant-a"},
			audit:      db.Audit{ID: "audit-1"},
			evaluation: db.Evaluation{ID: "eval-1", TenantID: "tenant-b"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanViewAudit(ctx, tt.user, tt.audit, tt.evaluation)
			if result != tt.expected {
				t.Errorf("CanViewAudit() = %v, want %v", result, tt.expected)
			}
		})
	}
}
