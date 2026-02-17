package view

import "testing"

func TestNewPagination(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		total     int
		perPage   int
		wantPages int
	}{
		{"Primeira página", 1, 25, 10, 3},
		{"Página negativa", -1, 25, 10, 3},
		{"Muitos itens", 1, 100, 10, 10},
		{"Zero itens", 1, 0, 10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPagination(tt.page, tt.total, tt.perPage)
			if p.TotalPages() != tt.wantPages {
				t.Errorf("TotalPages() = %v, want %v", p.TotalPages(), tt.wantPages)
			}
		})
	}
}

func TestPagination_Navigation(t *testing.T) {
	p := NewPagination(2, 30, 10) // Página 2 de 3

	if !p.HasPrevious() {
		t.Error("Deveria ter página anterior")
	}
	if !p.HasNext() {
		t.Error("Deveria ter próxima página")
	}
	if p.PreviousPage() != 1 {
		t.Errorf("PreviousPage() = %v, want 1", p.PreviousPage())
	}
	if p.NextPage() != 3 {
		t.Errorf("NextPage() = %v, want 3", p.NextPage())
	}
}
