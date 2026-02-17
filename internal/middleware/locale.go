package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/PauloHFS/goth/internal/contextkeys"
)

func Locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Verificar Cookie (preferência manual)
		locale := "pt"
		cookie, err := r.Cookie("lang")
		if err == nil {
			locale = cookie.Value
		} else {
			// 2. Verificar Header Accept-Language
			accept := r.Header.Get("Accept-Language")
			if strings.HasPrefix(accept, "en") {
				locale = "en"
			}
		}

		ctx := context.WithValue(r.Context(), contextkeys.LocaleKey, locale)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
