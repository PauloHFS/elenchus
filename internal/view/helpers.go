package view

import (
	"context"
	"github.com/PauloHFS/goth/internal/contextkeys"
)

// CSRFToken retorna o token do contexto
func CSRFToken(ctx context.Context) string {
	if token, ok := ctx.Value(contextkeys.CSRFTokenKey).(string); ok {
		return token
	}
	return ""
}
