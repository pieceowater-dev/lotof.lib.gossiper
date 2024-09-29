package gossiper

// FilterSortByEnum defines sorting options.
type FilterSortByEnum string

const (
	// ASC represents ascending sort order.
	ASC FilterSortByEnum = "ASC"
	// DESC represents descending sort order.
	DESC FilterSortByEnum = "DESC"
)

// FilterPaginationLengthEnum defines pagination length options.
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

// Sort defines the structure for sorting data based on a field and order.
type Sort[T any] struct {
	By    FilterSortByEnum `json:"by,omitempty"`    // Sort order: ASC or DESC
	Field string           `json:"field,omitempty"` // The field to sort by
}

// Paginated defines pagination properties for data requests.
type Paginated struct {
	Length FilterPaginationLengthEnum `json:"length,omitempty"` // Number of items per page
	Page   int                        `json:"page,omitempty"`   // Current page number
}

// DefaultFilter defines a filter structure with search, sort, and pagination options.
type DefaultFilter[T any] struct {
	Search     string    `json:"search,omitempty"`     // Search query string
	Sort       Sort[T]   `json:"sort,omitempty"`       // Sort parameters
	Pagination Paginated `json:"pagination,omitempty"` // Pagination parameters
}

// NewDefaultFilter - Constructor for DefaultFilter with default pagination values.
func NewDefaultFilter[T any]() DefaultFilter[T] {
	return DefaultFilter[T]{
		Sort: Sort[T]{
			Field: "id",
			By:    DESC,
		},
		Pagination: Paginated{
			Page:   1,
			Length: TEN,
		},
	}
}
