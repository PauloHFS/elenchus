package policies

import (
	"context"
	"fmt"

	"github.com/PauloHFS/elenchus/internal/db"
)

// EvaluationContext representa o contexto para avaliação de políticas
type EvaluationContext struct {
	User         db.User
	Evaluation   db.Evaluation
	TenantID     string
	Action       string
	ResourceType string
}

// Action representa uma ação que pode ser realizada
type Action string

const (
	ActionView   Action = "view"
	ActionEdit   Action = "edit"
	ActionDelete Action = "delete"
	ActionAudit  Action = "audit"
)

// ResourceType representa o tipo de recurso
type ResourceType string

const (
	ResourceEvaluation ResourceType = "evaluation"
	ResourceIteration  ResourceType = "iteration"
	ResourceAudit      ResourceType = "audit"
)

// CanAccessEvaluation verifica se o usuário pode acessar uma avaliação
// Política baseada em atributos (ABAC):
// - Admins podem acessar todas as avaliações
// - Usuários podem acessar apenas avaliações do seu tenant
func CanAccessEvaluation(ctx context.Context, user db.User, evaluation db.Evaluation) bool {
	// Admin tem acesso total
	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return true
	}

	// Usuário deve pertencer ao mesmo tenant
	return user.TenantID == evaluation.TenantID
}

// CanCreateEvaluation verifica se o usuário pode criar uma nova avaliação
// Política:
// - Usuários autenticados podem criar avaliações no seu tenant
// - Admins podem criar em qualquer tenant (se aplicável)
func CanCreateEvaluation(ctx context.Context, user db.User, tenantID string) bool {
	if user.ID == 0 {
		return false // Usuário não autenticado
	}

	// Admin pode criar em qualquer tenant
	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return true
	}

	// Usuário normal só pode criar no seu próprio tenant
	return user.TenantID == tenantID
}

// CanDeleteEvaluation verifica se o usuário pode deletar uma avaliação
// Política mais restritiva:
// - Apenas admins podem deletar avaliações
// - Ou o criador da avaliação (se for o mesmo usuário)
func CanDeleteEvaluation(ctx context.Context, user db.User, evaluation db.Evaluation) bool {
	// Admin tem acesso total
	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return true
	}

	// Apenas o criador pode deletar (se houver controle de userID)
	if evaluation.UserID != 0 && user.ID == evaluation.UserID {
		return true
	}

	return false
}

// CanViewAudit verifica se o usuário pode visualizar auditorias
// Política:
// - Admins podem visualizar todas as auditorias
// - Usuários podem visualizar auditorias de avaliações do seu tenant
func CanViewAudit(ctx context.Context, user db.User, audit db.Audit, evaluation db.Evaluation) bool {
	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return true
	}

	// Verificar se a auditoria pertence a uma avaliação do tenant do usuário
	return evaluation.TenantID == user.TenantID
}

// CanViewIteration verifica se o usuário pode visualizar iterações
// Política:
// - Admins podem visualizar todas as iterações
// - Usuários podem visualizar iterações de avaliações do seu tenant
func CanViewIteration(ctx context.Context, user db.User, iteration db.Iteration, evaluation db.Evaluation) bool {
	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return true
	}

	return evaluation.TenantID == user.TenantID
}

// CheckEvaluationAccess é uma função genérica para verificar acesso a avaliações
// Retorna erro se o acesso for negado
func CheckEvaluationAccess(ctx context.Context, user db.User, evaluation db.Evaluation, action Action) error {
	if user.ID == 0 {
		return fmt.Errorf("unauthorized: user not authenticated")
	}

	switch action {
	case ActionView:
		if !CanAccessEvaluation(ctx, user, evaluation) {
			return fmt.Errorf("forbidden: user cannot view this evaluation")
		}
	case ActionEdit:
		if !CanAccessEvaluation(ctx, user, evaluation) {
			return fmt.Errorf("forbidden: user cannot edit this evaluation")
		}
		// Não permitir edição de avaliações completadas
		if evaluation.Status == "completed" || evaluation.Status == "failed" {
			return fmt.Errorf("forbidden: cannot modify completed/failed evaluations")
		}
	case ActionDelete:
		if !CanDeleteEvaluation(ctx, user, evaluation) {
			return fmt.Errorf("forbidden: user cannot delete this evaluation")
		}
	case ActionAudit:
		if user.RoleID != "admin" && user.RoleID != "administrator" {
			return fmt.Errorf("forbidden: only admins can perform audit actions")
		}
	}

	return nil
}

// CheckTenantAccess verifica se o usuário tem acesso ao tenant especificado
func CheckTenantAccess(ctx context.Context, user db.User, tenantID string) error {
	if user.ID == 0 {
		return fmt.Errorf("unauthorized: user not authenticated")
	}

	if user.RoleID == "admin" || user.RoleID == "administrator" {
		return nil // Admin tem acesso a todos os tenants
	}

	if user.TenantID != tenantID {
		return fmt.Errorf("forbidden: user does not have access to this tenant")
	}

	return nil
}
