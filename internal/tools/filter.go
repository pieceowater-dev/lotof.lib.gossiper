package gossiper

import t "github.com/pieceowater-dev/lotof.lib.gossiper/types"

const (
	// ASC represents ascending sort order.
	ASC t.FilterSortByEnum = "ASC"
	// DESC represents descending sort order.
	DESC t.FilterSortByEnum = "DESC"
)

const (
	TEN          t.FilterPaginationLengthEnum = 10
	FIFTEEN      t.FilterPaginationLengthEnum = 15
	TWENTY       t.FilterPaginationLengthEnum = 20
	TWENTY_FIVE  t.FilterPaginationLengthEnum = 25
	THIRTY       t.FilterPaginationLengthEnum = 30
	THIRTY_FIVE  t.FilterPaginationLengthEnum = 35
	FORTY        t.FilterPaginationLengthEnum = 40
	FORTY_FIVE   t.FilterPaginationLengthEnum = 45
	FIFTY        t.FilterPaginationLengthEnum = 50
	FIFTY_FIVE   t.FilterPaginationLengthEnum = 55
	SIXTY        t.FilterPaginationLengthEnum = 60
	SIXTY_FIVE   t.FilterPaginationLengthEnum = 65
	SEVENTY      t.FilterPaginationLengthEnum = 70
	SEVENTY_FIVE t.FilterPaginationLengthEnum = 75
	EIGHTY       t.FilterPaginationLengthEnum = 80
	EIGHTY_FIVE  t.FilterPaginationLengthEnum = 85
	NINETY       t.FilterPaginationLengthEnum = 90
	NINETY_FIVE  t.FilterPaginationLengthEnum = 95
	ONE_HUNDRED  t.FilterPaginationLengthEnum = 100
)

// NewDefaultFilter - Constructor for DefaultFilter with default pagination values.
func NewDefaultFilter[T any]() t.DefaultFilter[T] {
	return t.DefaultFilter[T]{
		Sort: t.Sort[T]{
			Field: "id",
			By:    DESC,
		},
		Pagination: t.Paginated{
			Page:   1,
			Length: TEN,
		},
	}
}
