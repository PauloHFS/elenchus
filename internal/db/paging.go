package db

// PagingParams define os parâmetros básicos de entrada
type PagingParams struct {
	Page    int
	PerPage int
}

func (p PagingParams) Offset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	return (p.Page - 1) * p.Limit()
}

func (p PagingParams) Limit() int {
	if p.PerPage < 1 {
		p.PerPage = 10
	}
	return p.PerPage
}

// PagedResult encapsula os dados e os metadados da página
type PagedResult[T any] struct {
	Items       []T
	TotalItems  int
	CurrentPage int
	PerPage     int
}

func (p PagedResult[T]) TotalPages() int {
	if p.PerPage == 0 {
		return 0
	}
	return (p.TotalItems + p.PerPage - 1) / p.PerPage
}
