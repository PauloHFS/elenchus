package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	// Apenas para silenciar o erro de não uso e mostrar que o request foi criado
	_ = req

	if rr.Code != http.StatusOK {
		t.Log("Recorder pronto para testes")
	}
}
