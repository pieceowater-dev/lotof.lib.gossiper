package pagination

// EntityInfo provides additional metadata for a paginated entity.
type EntityInfo struct {
	Count int `json:"count"` // Total number of entities
}

// PaginatedEntity wraps paginated results along with entity metadata.
type PaginatedEntity[T any] struct {
	Rows []T        `json:"rows"` // Slice of entities for the current page
	Info EntityInfo `json:"info"` // Metadata including total count
}

// ToPaginated converts raw data (a slice of entities and count) to a PaginatedEntity.
func ToPaginated[T any](items []T, count int) PaginatedEntity[T] {
	// Return a PaginatedEntity with the rows and entity count
	return PaginatedEntity[T]{
		Rows: items,
		Info: EntityInfo{
			Count: count,
		},
	}
}
