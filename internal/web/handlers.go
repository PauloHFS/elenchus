package web

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/PauloHFS/elenchus/internal/config"
	"github.com/PauloHFS/elenchus/internal/contextkeys"
	"github.com/PauloHFS/elenchus/internal/db"
	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/PauloHFS/elenchus/internal/middleware"
	"github.com/PauloHFS/elenchus/internal/policies"
	"github.com/PauloHFS/elenchus/internal/routes"
	"github.com/PauloHFS/elenchus/internal/service"
	"github.com/PauloHFS/elenchus/internal/sse"
	"github.com/PauloHFS/elenchus/internal/view"
	"github.com/PauloHFS/elenchus/internal/view/pages"
	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
)

type HandlerDeps struct {
	DB             *sql.DB
	Queries        *db.Queries
	SessionManager *scs.SessionManager
	Logger         *slog.Logger
	Config         *config.Config
	SSEBroker      *sse.Broker
}

// AppHandler é um tipo customizado que permite retornar erros dos handlers
type AppHandler func(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error

// Handle envolve nosso AppHandler para conformidade com http.HandlerFunc
func Handle(deps HandlerDeps, h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(deps, w, r); err != nil {
			// Aqui centralizamos o log de erro estruturado
			deps.Logger.Error("request failed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Any("error", err),
			)

			// Decidir o que mostrar ao usuário
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func RegisterRoutes(mux *http.ServeMux, deps HandlerDeps) {
	// Auth Handlers
	mux.Handle("GET "+routes.Login, templ.Handler(pages.Login("")))
	mux.Handle("GET "+routes.Register, templ.Handler(pages.Register("")))

	mux.HandleFunc("POST "+routes.Register, Handle(deps, handleRegister))
	mux.HandleFunc("GET "+routes.ForgotPassword, func(w http.ResponseWriter, r *http.Request) {
		templ.Handler(pages.ForgotPassword("")).ServeHTTP(w, r)
	})
	mux.HandleFunc("POST "+routes.ForgotPassword, Handle(deps, handleForgotPassword))
	mux.HandleFunc("GET "+routes.ResetPassword, func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		templ.Handler(pages.ResetPassword(token, "")).ServeHTTP(w, r)
	})
	mux.HandleFunc("POST "+routes.ResetPassword, Handle(deps, handleResetPassword))
	mux.HandleFunc("GET "+routes.VerifyEmail, Handle(deps, handleVerifyEmail))
	mux.HandleFunc("POST "+routes.Login, Handle(deps, handleLogin))
	mux.HandleFunc("POST "+routes.Logout, Handle(deps, handleLogout))

	// Protected Routes
	mux.Handle("GET "+routes.Dashboard, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleDashboard)))
	mux.Handle("POST /profile/avatar", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleAvatarUpload)))
	mux.Handle("POST /dashboard/test-job", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleTestJob)))

	// Evaluation Routes
	mux.Handle("GET "+routes.EvaluationsPage, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleEvaluationsPage)))
	mux.Handle("POST "+routes.EvaluationStart, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleStartEvaluation)))
	mux.Handle("GET /sse", deps.SSEBroker.Handler()) // SSE endpoint for HTMX
	mux.Handle("GET "+routes.EvaluationResult, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleLoadEvaluationResult)))
	mux.Handle("GET "+routes.EvaluationsList, middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleListEvaluations)))
	mux.Handle("GET /evaluations/history", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleListEvaluations)))
	mux.Handle("GET /evaluations/active", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleActiveEvaluations)))
	mux.Handle("GET /evaluations/status/{id}", middleware.RequireAuth(deps.SessionManager, deps.Queries, Handle(deps, handleEvaluationStatus)))

	// Public Routes
	mux.HandleFunc("GET "+routes.Home, func(w http.ResponseWriter, r *http.Request) {
		logging.AddToEvent(r.Context(), slog.String("business_unit", "marketing"))
		_, _ = w.Write([]byte("GOTH Stack Running"))
	})
}

// --- Handler Implementations ---

func handleRegister(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")
	password := r.FormValue("password")

	_, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})
	if err == nil {
		templ.Handler(pages.Register("Este e-mail já está em uso")).ServeHTTP(w, r)
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	_, err = qtx.CreateUser(r.Context(), db.CreateUserParams{
		TenantID:     "default",
		Email:        email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	tokenBytes := make([]byte, 32)
	if _, err := crypto_rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	if err := qtx.UpsertEmailVerification(r.Context(), db.UpsertEmailVerificationParams{
		Email:     email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return fmt.Errorf("failed to create verification: %w", err)
	}

	jobPayload, err := json.Marshal(map[string]string{
		"email": email,
		"token": token,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	if _, err := qtx.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "send_verification_email",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit registration: %w", err)
	}

	http.Redirect(w, r, routes.Login+"?message=Conta criada! Verifique seu e-mail.", http.StatusSeeOther)
	return nil
}

func handleForgotPassword(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")
	_, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})
	if err != nil {
		templ.Handler(pages.ForgotPassword("Se o e-mail existir, um link será enviado.")).ServeHTTP(w, r)
		return nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := crypto_rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	if err := qtx.UpsertPasswordReset(r.Context(), db.UpsertPasswordResetParams{
		Email:     email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return fmt.Errorf("failed to create password reset: %w", err)
	}

	jobPayload, err := json.Marshal(map[string]string{
		"email": email,
		"token": token,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}
	if _, err := qtx.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "send_password_reset_email",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit forgot password: %w", err)
	}

	templ.Handler(pages.ForgotPassword("Se o e-mail existir, um link será enviado.")).ServeHTTP(w, r)
	return nil
}

func handleResetPassword(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	token := r.FormValue("token")
	password := r.FormValue("password")

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	reset, err := deps.Queries.GetPasswordResetByToken(r.Context(), tokenHash)
	if err != nil || reset.ExpiresAt.Before(time.Now()) {
		templ.Handler(pages.ResetPassword(token, "Link inválido ou expirado")).ServeHTTP(w, r)
		return nil
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	err = qtx.UpdateUserPassword(r.Context(), db.UpdateUserPasswordParams{
		PasswordHash: string(newHash),
		Email:        reset.Email,
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if err := qtx.DeletePasswordReset(r.Context(), reset.Email); err != nil {
		deps.Logger.Warn("failed to delete password reset token", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit password reset: %w", err)
	}

	http.Redirect(w, r, routes.Login+"?message=Senha alterada com sucesso", http.StatusSeeOther)
	return nil
}

func handleVerifyEmail(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, routes.Login+"?error=token_invalido", http.StatusSeeOther)
		return nil
	}

	verification, err := deps.Queries.GetEmailVerificationByToken(r.Context(), token)
	if err != nil || verification.ExpiresAt.Before(time.Now()) {
		http.Redirect(w, r, routes.Login+"?error=token_expirado", http.StatusSeeOther)
		return nil
	}

	tx, err := deps.DB.BeginTx(r.Context(), nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := deps.Queries.WithTx(tx)

	err = qtx.VerifyUser(r.Context(), verification.Email)
	if err != nil {
		return fmt.Errorf("failed to verify user: %w", err)
	}

	if err := qtx.DeleteEmailVerification(r.Context(), verification.Email); err != nil {
		deps.Logger.Warn("failed to delete email verification token", "error", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit email verification: %w", err)
	}

	http.Redirect(w, r, routes.Login+"?message=E-mail verificado com sucesso", http.StatusSeeOther)
	return nil
}

func handleLogin(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := deps.Queries.GetUserByEmail(r.Context(), db.GetUserByEmailParams{
		TenantID: "default",
		Email:    email,
	})

	if err != nil {
		templ.Handler(pages.Login("Usuário ou senha inválidos")).ServeHTTP(w, r)
		return nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		templ.Handler(pages.Login("Usuário ou senha inválidos")).ServeHTTP(w, r)
		return nil
	}

	deps.SessionManager.Put(r.Context(), "user_id", user.ID)
	http.Redirect(w, r, routes.Dashboard, http.StatusSeeOther)
	return nil
}

func handleLogout(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	if err := deps.SessionManager.Destroy(r.Context()); err != nil {
		return fmt.Errorf("failed to destroy session: %w", err)
	}
	http.Redirect(w, r, routes.Login, http.StatusSeeOther)
	return nil
}

func handleDashboard(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, _ := r.Context().Value(contextkeys.UserContextKey).(db.User)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")

	paging := db.PagingParams{
		Page:    page,
		PerPage: 5,
	}

	users, err := deps.Queries.ListUsersPaginated(r.Context(), db.ListUsersPaginatedParams{
		TenantID: "default",
		Column2:  sql.NullString{String: search, Valid: true},
		Column3:  sql.NullString{String: search, Valid: true},
		Limit:    int64(paging.Limit()),
		Offset:   int64(paging.Offset()),
	})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	totalUsers, err := deps.Queries.CountUsers(r.Context(), "default")
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	result := db.PagedResult[db.User]{
		Items:       users,
		TotalItems:  int(totalUsers),
		CurrentPage: paging.Page,
		PerPage:     paging.PerPage,
	}

	pagHelper := view.NewPagination(result.CurrentPage, result.TotalItems, result.PerPage)
	templ.Handler(pages.Dashboard(user, result.Items, pagHelper)).ServeHTTP(w, r)
	return nil
}

// handleTestJob inicia um job assíncrono de teste que notifica via SSE
func handleTestJob(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	// Retorna status inicial
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(pages.JobStatusNotification("running", "Job iniciado... processando em background")).ServeHTTP(w, r)

	// Inicia job em background
	go func() {
		// Envia progresso via SSE
		deps.SSEBroker.SendHTML("user", fmt.Sprint(user.ID), "job_progress",
			`<div class="bg-blue-50 border-l-4 border-blue-500 p-4 rounded mb-2">
				<div class="flex items-center">
					<svg class="animate-spin h-5 w-5 text-blue-700 mr-3" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
					</svg>
					<p class="text-blue-700 font-medium">Processando... 50%</p>
				</div>
			</div>`)

		// Simula processamento (5 segundos)
		time.Sleep(5 * time.Second)

		// Envia notificação de conclusão via SSE
		deps.SSEBroker.SendHTML("user", fmt.Sprint(user.ID), "job_completed",
			`<div class="bg-green-50 border-l-4 border-green-500 p-4 rounded mb-2">
				<div class="flex items-center">
					<svg class="h-5 w-5 text-green-700 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
					</svg>
					<p class="text-green-700 font-medium">✅ Job completado com sucesso! Processamento levou ~5 segundos.</p>
				</div>
			</div>`)

		// Log para debug
		fmt.Printf("✅ Test job completed for user %d\n", user.ID)
	}()

	return nil
}

func handleAvatarUpload(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		return fmt.Errorf("failed to parse multipart form: %w", err)
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		http.Error(w, "invalid file", http.StatusBadRequest)
		return nil
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", user.ID, ext)
	dstPath := filepath.Join("storage", "avatars", filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	avatarURL := "/storage/avatars/" + filename
	if err := deps.Queries.UpdateUserAvatar(r.Context(), db.UpdateUserAvatarParams{
		AvatarUrl: sql.NullString{String: avatarURL, Valid: true},
		ID:        user.ID,
	}); err != nil {
		deps.Logger.Warn("failed to update avatar in database", "error", err)
	}

	jobPayload, _ := json.Marshal(map[string]string{"image": avatarURL})
	if _, err := deps.Queries.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: fmt.Sprintf("%d", user.ID), Valid: true},
		Type:     "process_ai",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		deps.Logger.Warn("failed to create AI processing job", "error", err)
	}

	http.Redirect(w, r, routes.Dashboard, http.StatusSeeOther)
	return nil
}

// --- Evaluation Handlers ---

func handleEvaluationsPage(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	templ.Handler(pages.Evaluations(user)).ServeHTTP(w, r)
	return nil
}

func handleStartEvaluation(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	prompt := r.FormValue("prompt")
	if prompt == "" {
		http.Error(w, "Prompt é obrigatório", http.StatusBadRequest)
		return nil
	}

	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	evalService, err := service.NewEvaluationService(deps.Queries, deps.SSEBroker)
	if err != nil {
		return fmt.Errorf("failed to create evaluation service: %w", err)
	}
	evalID, err := evalService.StartEvaluation(r.Context(), user.TenantID, user.ID, prompt)
	if err != nil {
		return fmt.Errorf("failed to start evaluation: %w", err)
	}

	// Return HTML with SSE connection using HTMX SSE extension
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(pages.SSEEvaluationContainer(evalID)).ServeHTTP(w, r)
	return nil
}

// handleLoadEvaluationResult renders the final evaluation result
func handleLoadEvaluationResult(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	evalID := r.PathValue("id")
	if evalID == "" {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return nil
	}

	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	// Check if evaluation exists
	eval, err := deps.Queries.GetEvaluationByID(r.Context(), evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Policy check: User can only access evaluations from their tenant
	if eval.TenantID != user.TenantID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	// Check if still processing or retrying
	if eval.Status == "pending" || eval.Status == "processing" || eval.Status == "retrying" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4"
			hx-get="/htmx/evaluations/`+evalID+`/result"
			hx-trigger="load delay:3s"
			hx-swap="outerHTML">
			<div class="flex items-center">
				<svg class="animate-spin h-5 w-5 text-yellow-800 mr-3" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<p class="text-yellow-800">Avaliação ainda processando...</p>
			</div>
			<p class="text-sm text-yellow-700 mt-2">Status: { eval.Status }</p>
		</div>`)
		return nil
	}

	// Check if failed
	if eval.Status == "failed" {
		w.Header().Set("Content-Type", "text/html")
		errorMsg := "Avaliação falhou. Tente novamente."
		if eval.ErrorMessage.Valid && eval.ErrorMessage.String != "" {
			errorMsg = eval.ErrorMessage.String
		}
		templ.Handler(pages.SSEError(errorMsg)).ServeHTTP(w, r)
		return nil
	}

	// Completed - get iterations and audit
	iterations, err := deps.Queries.GetIterationsByEvaluation(r.Context(), evalID)
	if err != nil {
		return fmt.Errorf("failed to get iterations: %w", err)
	}

	audit, err := deps.Queries.GetAuditByEvaluation(r.Context(), evalID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Audit not ready yet, show waiting message
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4"
				hx-get="/htmx/evaluations/`+evalID+`/result"
				hx-trigger="load delay:2s"
				hx-swap="outerHTML">
				<p class="text-yellow-800">⏳ Finalizando auditoria...</p>
			</div>`)
			return nil
		}
		return fmt.Errorf("failed to get audit: %w", err)
	}

	// Render result using templ component
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(pages.EvaluationResult(eval, iterations, audit)).ServeHTTP(w, r)
	return nil
}

// handleListEvaluations lista as avaliações do usuário
func handleListEvaluations(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	// Policy check: User can only list evaluations from their tenant
	if err := policies.CheckTenantAccess(r.Context(), user, user.TenantID); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	evaluations, err := deps.Queries.ListEvaluationsPaginated(r.Context(), db.ListEvaluationsPaginatedParams{
		TenantID: user.TenantID,
		UserID:   user.ID,
		Limit:    10,
		Offset:   int64((page - 1) * 10),
	})
	if err != nil {
		return fmt.Errorf("failed to list evaluations: %w", err)
	}

	total, err := deps.Queries.CountEvaluations(r.Context(), db.CountEvaluationsParams{
		TenantID: user.TenantID,
		UserID:   user.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to count evaluations: %w", err)
	}

	// Renderizar lista
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div class="evaluations-list">`)
	fmt.Fprintf(w, `<h2>Suas Avaliações (%d)</h2>`, total)
	for _, eval := range evaluations {
		statusClass := "status-" + eval.Status
		fmt.Fprintf(w, `<div class="evaluation-item %s"><a href="/htmx/evaluations/%s/result">%s - %s</a></div>`,
			statusClass, eval.ID, eval.CreatedAt.Time.Format("2006-01-02 15:04"), eval.Status)
	}
	fmt.Fprint(w, `</div>`)

	return nil
}

// handleActiveEvaluations retorna avaliações ativas/em retry do usuário
func handleActiveEvaluations(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	// Policy check
	if err := policies.CheckTenantAccess(r.Context(), user, user.TenantID); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	// Busca avaliações ativas (processing ou retrying)
	evaluations, err := deps.Queries.ListEvaluationsByStatus(r.Context(), db.ListEvaluationsByStatusParams{
		TenantID: user.TenantID,
		UserID:   user.ID,
		Status:   "processing",
		Status_2: "retrying",
	})
	if err != nil {
		return fmt.Errorf("failed to list active evaluations: %w", err)
	}

	if len(evaluations) == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return nil
	}

	// Renderiza lista de avaliações ativas
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(pages.ActiveEvaluationsList(evaluations)).ServeHTTP(w, r)
	return nil
}

// handleEvaluationStatus verifica status de uma avaliação específica e retorna componente apropriado
func handleEvaluationStatus(deps HandlerDeps, w http.ResponseWriter, r *http.Request) error {
	user, ok := r.Context().Value(contextkeys.UserContextKey).(db.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	evalID := r.PathValue("id")
	if evalID == "" {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return nil
	}

	// Policy check: verificar se usuário pode acessar esta avaliação
	eval, err := deps.Queries.GetEvaluationByID(r.Context(), evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	if eval.TenantID != user.TenantID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil
	}

	// Verifica status atual
	switch eval.Status {
	case "retrying":
		// Busca checkpoint pra ver info de retry
		checkpoint, err := deps.Queries.GetCheckpoint(r.Context(), evalID)
		if err != nil {
			// Sem checkpoint, mostra status genérico
			w.Header().Set("Content-Type", "text/html")
			templ.Handler(pages.SSERetrying(evalID, eval.RetryCount, "")).ServeHTTP(w, r)
			return nil
		}

		nextRetryAt := ""
		if checkpoint.NextRetryAt.Valid {
			nextRetryAt = checkpoint.NextRetryAt.Time.Format("15:04:05")
		}

		w.Header().Set("Content-Type", "text/html")
		templ.Handler(pages.SSERetrying(evalID, checkpoint.RetryCount, nextRetryAt)).ServeHTTP(w, r)
		return nil

	case "processing":
		// Ainda tá processando, retorna status de processamento
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
			<p class="text-yellow-800">⏳ Processando avaliação...</p>
		</div>`)
		return nil

	case "completed":
		// Completou! Retorna resultado
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="bg-green-50 border border-green-200 rounded-lg p-4"
			hx-get="/htmx/evaluations/`+evalID+`/result"
			hx-trigger="load"
			hx-swap="outerHTML">
			<p class="text-green-800">✅ Avaliação completa! Carregando resultado...</p>
		</div>`)
		return nil

	case "failed":
		// Falhou
		w.Header().Set("Content-Type", "text/html")
		errorMsg := "Avaliação falhou. Tente novamente."
		if eval.ErrorMessage.Valid && eval.ErrorMessage.String != "" {
			errorMsg = eval.ErrorMessage.String
		}
		templ.Handler(pages.SSEError(errorMsg)).ServeHTTP(w, r)
		return nil

	default:
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return nil
	}
}
