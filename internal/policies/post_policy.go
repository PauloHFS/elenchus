package policies

import "github.com/PauloHFS/goth/internal/db"

// CanEditPost implementa lógica ABAC para edição de conteúdo.
func CanEditPost(user db.User, post db.Post) bool {
	if user.RoleID == "admin" {
		return true
	}
	return user.ID == post.UserID
}
