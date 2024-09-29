package gossiper

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
// The first element of data is expected to be the list of entities and the second element is the total count.
func ToPaginated[T any](data []any) PaginatedEntity[T] {
	// Convert the raw data[0] to a slice of T
	items := make([]T, len(data[0].([]any)))
	for i, v := range data[0].([]any) {
		items[i] = v.(T) // Type assertion
	}

	// Return a PaginatedEntity with the rows and entity count
	return PaginatedEntity[T]{
		Rows: items,
		Info: EntityInfo{
			Count: data[1].(int), // Assuming the second element is the total count
		},
	}
}
