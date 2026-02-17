package middleware

import (
	"context"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/routes"
	"github.com/alexedwards/scs/v2"
)

func RequireAuth(sm *scs.SessionManager, queries *db.Queries, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := sm.GetInt64(r.Context(), "user_id")
		if userID == 0 {
			redirectLogin(w, r)
			return
		}

		// Buscar usuário completo e colocar no contexto
		// Nota: Em apps de altíssimo tráfego, você poderia colocar o usuário no cache (Redis/LRU)
		user, err := queries.GetUserByID(r.Context(), userID)
		if err != nil {
			_ = sm.Destroy(r.Context())
			redirectLogin(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), contextkeys.UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func redirectLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", routes.Login)
	} else {
		http.Redirect(w, r, routes.Login, http.StatusSeeOther)
	}
}

// GetUser recupera o usuário do contexto de forma segura
func GetUser(ctx context.Context) (db.User, bool) {
	user, ok := ctx.Value(contextkeys.UserContextKey).(db.User)
	return user, ok
}
