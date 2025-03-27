package generic

type PaginatedResult[T any] struct {
	Rows []T `json:"rows"`
	Info struct {
		Count      int        `json:"count"`
		Pagination Pagination `json:"pagination"`
	} `json:"info"`
}

func NewPaginatedResult[T any](rows []T, count int, pagination Pagination) PaginatedResult[T] {
	return PaginatedResult[T]{
		Rows: rows,
		Info: struct {
			Count      int        `json:"count"`
			Pagination Pagination `json:"pagination"`
		}{
			Count:      count,
			Pagination: pagination,
		},
	}
}
