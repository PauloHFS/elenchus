package view

import "math"

type Pagination struct {
	CurrentPage int
	TotalItems  int
	PerPage     int
}

func NewPagination(page, total, perPage int) Pagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	return Pagination{
		CurrentPage: page,
		TotalItems:  total,
		PerPage:     perPage,
	}
}

func (p Pagination) TotalPages() int {
	return int(math.Ceil(float64(p.TotalItems) / float64(p.PerPage)))
}

func (p Pagination) HasPrevious() bool {
	return p.CurrentPage > 1
}

func (p Pagination) HasNext() bool {
	return p.CurrentPage < p.TotalPages()
}

func (p Pagination) PreviousPage() int {
	if p.HasPrevious() {
		return p.CurrentPage - 1
	}
	return 1
}

func (p Pagination) NextPage() int {
	if p.HasNext() {
		return p.CurrentPage + 1
	}
	return p.TotalPages()
}
