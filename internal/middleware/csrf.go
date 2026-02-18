package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/PauloHFS/elenchus/internal/contextkeys"
	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/justinas/nosurf"
)

// CSRFDisabled verifica se CSRF está desabilitado via variável de ambiente
func CSRFDisabled() bool {
	return os.Getenv("DISABLE_CSRF") == "true" || os.Getenv("DISABLE_CSRF") == "1"
}

// CSRFWithContext retorna um handler que processa CSRF (nosurf),
// ou passa direto se DISABLE_CSRF estiver setado
func CSRFWithContext(next http.Handler) http.Handler {
	// Se CSRF estiver desabilitado, apenas injeta um token fake e passa adiante
	if CSRFDisabled() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Injeta um token fake no contexto
			ctx := context.WithValue(r.Context(), contextkeys.CSRFTokenKey, "disabled")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// Cria o handler CSRF do nosurf
	csrfHandler := nosurf.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Após o nosurf processar, o token está disponível
		token := nosurf.Token(r)
		// Injeta o token no contexto
		ctx := context.WithValue(r.Context(), contextkeys.CSRFTokenKey, token)
		// Chama o próximo handler com o novo contexto
		next.ServeHTTP(w, r.WithContext(ctx))
	}))

	// Configura o cookie com MaxAge para persistir
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 horas
	})

	// Configura o handler de erro
	csrfHandler.SetFailureHandler(http.HandlerFunc(CSRFErrorHandler))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sse" {
			next.ServeHTTP(w, r)
			return
		}
		csrfHandler.ServeHTTP(w, r)
	})
}

// CSRFErrorHandler cria um handler para logging de falhas CSRF
func CSRFErrorHandler(w http.ResponseWriter, r *http.Request) {
	logger := logging.Get()

	// Se CSRF estiver desabilitado, não deveria chegar aqui
	if CSRFDisabled() {
		http.Error(w, "CSRF check error", http.StatusInternalServerError)
		return
	}

	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "unknown"
	}

	formToken := r.FormValue("gorilla.csrf.Token")
	if formToken == "" {
		formToken = r.FormValue("csrf_token")
	}

	var cookieNames []string
	var csrfCookieValue string
	for _, cookie := range r.Cookies() {
		cookieNames = append(cookieNames, cookie.Name)
		if cookie.Name == "csrf_token" {
			csrfCookieValue = truncateString(cookie.Value, 20)
		}
	}

	logger.Error("csrf validation failed",
		slog.String("request_id", requestID),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
		slog.String("referer", r.Header.Get("Referer")),
		slog.String("csrf_token_header", r.Header.Get("X-CSRF-Token")),
		slog.String("csrf_token_form", truncateString(formToken, 30)),
		slog.String("csrf_cookie_value", csrfCookieValue),
		slog.Any("all_cookies", cookieNames),
		slog.String("cookie_header", r.Header.Get("Cookie")),
	)

	http.Error(w, "Invalid CSRF token", http.StatusBadRequest)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
