package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/PauloHFS/elenchus/internal/logging"
)

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger := logging.Get()
				logger.Error("panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)

				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Internal Server Error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
