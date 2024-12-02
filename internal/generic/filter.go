package generic

type Filter[T any] struct {
	Search     string  `json:"search"`
	Sort       Sort[T] `json:"sort"`
	Pagination Pagination
}

func NewFilter[T any](search string, sort Sort[T], pagination Pagination) Filter[T] {
	return Filter[T]{
		Search:     search,
		Sort:       sort,
		Pagination: pagination,
	}
}
