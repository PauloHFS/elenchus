package policies

import (
	"github.com/PauloHFS/goth/internal/db"
	"testing"
)

func TestCanEditPost(t *testing.T) {
	admin := db.User{ID: 1, RoleID: "admin"}
	author := db.User{ID: 2, RoleID: "user"}
	other := db.User{ID: 3, RoleID: "user"}

	post := db.Post{ID: 10, UserID: 2}

	tests := []struct {
		name     string
		user     db.User
		post     db.Post
		expected bool
	}{
		{"Admin pode editar qualquer post", admin, post, true},
		{"Autor pode editar seu post", author, post, true},
		{"Outro usuário não pode editar", other, post, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanEditPost(tt.user, tt.post)
			if result != tt.expected {
				t.Errorf("falha em %s: esperado %v, obtido %v", tt.name, tt.expected, result)
			}
		})
	}
}
