package gossiper

const (
	// DefaultPageLength is used when a caller asks for no particular page
	// size (zero). It matches what the repositories already defaulted to.
	DefaultPageLength = 20
	// MaxPageLength is the platform ceiling for one page. It is deliberately
	// high rather than "nice": internal callers legitimately ask for 200
	// (catalog sync) and 1000 (plan lists), and the point of the ceiling is
	// to stop "give me the whole table", not to re-tune those flows.
	MaxPageLength = 1000
)

type integer interface {
	~int | ~int32 | ~int64
}

// ClampPageLength keeps a caller-supplied page size inside
// [1, MaxPageLength]. Zero (and anything negative, which GORM reads as "no
// limit at all") becomes DefaultPageLength. Take it before converting to a
// narrower type: a value above math.MaxInt32 turns into a negative int32,
// and a negative Limit is exactly the unbounded query this guards against.
func ClampPageLength[T integer](length T) T {
	if length <= 0 {
		return DefaultPageLength
	}
	if length > MaxPageLength {
		return MaxPageLength
	}
	return length
}

// ClampPage keeps a 1-based page number at 1 or above; page 0 and negative
// pages are the same request as page 1 and otherwise compute a negative
// OFFSET.
func ClampPage[T integer](page T) T {
	if page < 1 {
		return 1
	}
	return page
}

// ClampOffset keeps a row offset at zero or above.
func ClampOffset[T integer](offset T) T {
	if offset < 0 {
		return 0
	}
	return offset
}
