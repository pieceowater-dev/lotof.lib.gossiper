package gossiper

type EntityInfo struct {
	Count int `json:"count"`
}

type PaginatedEntity[T any] struct {
	Rows []T        `json:"rows"`
	Info EntityInfo `json:"info"`
}

func ToPaginated[T any](data []any) PaginatedEntity[T] {
	// Convert data[0] to []T
	items := make([]T, len(data[0].([]any)))
	for i, v := range data[0].([]any) {
		items[i] = v.(T) // Perform a type assertion for each element
	}

	return PaginatedEntity[T]{
		Rows: items,
		Info: EntityInfo{
			Count: data[1].(int), // Assuming count is at index 1
		},
	}
}
