package i18n

import (
	"context"
	"github.com/PauloHFS/goth/internal/contextkeys"
)

type Translation struct {
	Login     string
	Email     string
	Password  string
	Register  string
	Welcome   string
	Dashboard string
}

var ptBR = Translation{
	Login:     "Entrar",
	Email:     "E-mail",
	Password:  "Senha",
	Register:  "Registrar",
	Welcome:   "Bem-vindo",
	Dashboard: "Painel de Controle",
}

var enUS = Translation{
	Login:     "Login",
	Email:     "Email",
	Password:  "Password",
	Register:  "Register",
	Welcome:   "Welcome",
	Dashboard: "Dashboard",
}

// Get retorna as traduções baseadas no idioma do contexto
func Get(ctx context.Context) Translation {
	locale, _ := ctx.Value(contextkeys.LocaleKey).(string)
	switch locale {
	case "en":
		return enUS
	default:
		return ptBR
	}
}
