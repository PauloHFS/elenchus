package validator

import (
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validate verifica se a struct segue as regras das tags `validate`
func Validate(s interface{}) error {
	return validate.Struct(s)
}
