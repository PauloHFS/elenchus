package middleware

import (
	"context"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/justinas/nosurf"
)

func InjectCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := nosurf.Token(r)
		ctx := context.WithValue(r.Context(), contextkeys.CSRFTokenKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
