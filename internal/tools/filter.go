package gossiper

type FilterSortByEnum string

const (
	ASC  FilterSortByEnum = "ASC"
	DESC FilterSortByEnum = "DESC"
)

type FilterPaginationLengthEnum int

const (
	TEN          FilterPaginationLengthEnum = 10
	FIFTEEN      FilterPaginationLengthEnum = 15
	TWENTY       FilterPaginationLengthEnum = 20
	TWENTY_FIVE  FilterPaginationLengthEnum = 25
	THIRTY       FilterPaginationLengthEnum = 30
	THIRTY_FIVE  FilterPaginationLengthEnum = 35
	FORTY        FilterPaginationLengthEnum = 40
	FORTY_FIVE   FilterPaginationLengthEnum = 45
	FIFTY        FilterPaginationLengthEnum = 50
	FIFTY_FIVE   FilterPaginationLengthEnum = 55
	SIXTY        FilterPaginationLengthEnum = 60
	SIXTY_FIVE   FilterPaginationLengthEnum = 65
	SEVENTY      FilterPaginationLengthEnum = 70
	SEVENTY_FIVE FilterPaginationLengthEnum = 75
	EIGHTY       FilterPaginationLengthEnum = 80
	EIGHTY_FIVE  FilterPaginationLengthEnum = 85
	NINETY       FilterPaginationLengthEnum = 90
	NINETY_FIVE  FilterPaginationLengthEnum = 95
	ONE_HUNDRED  FilterPaginationLengthEnum = 100
)

type Sort[T any] struct {
	By    FilterSortByEnum `json:"by,omitempty"`
	Field string           `json:"field,omitempty"`
}

type Paginated struct {
	Length FilterPaginationLengthEnum `json:"length,omitempty"`
	Page   int                        `json:"page,omitempty"`
}

type DefaultFilter[T any] struct {
	Search     string    `json:"search,omitempty"`
	Sort       Sort[T]   `json:"sort,omitempty"`
	Pagination Paginated `json:"pagination,omitempty"`
}
