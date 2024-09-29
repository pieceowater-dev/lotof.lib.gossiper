package gossiper

// FilterPaginationLengthEnum defines pagination length options.
type FilterPaginationLengthEnum int

// FilterSortByEnum defines sorting options.
type FilterSortByEnum string

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
