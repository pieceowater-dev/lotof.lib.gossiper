package gossiper

import (
	"math"
	"testing"
)

func TestClampPageLength(t *testing.T) {
	cases := []struct {
		name string
		in   int64
		want int64
	}{
		{"unspecified takes the default", 0, DefaultPageLength},
		{"negative takes the default", -5, DefaultPageLength},
		{"ordinary value passes through", 50, 50},
		{"the ceiling itself passes through", MaxPageLength, MaxPageLength},
		{"above the ceiling is capped", MaxPageLength + 1, MaxPageLength},
		{"a huge value is capped", math.MaxInt32 + 1, MaxPageLength},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClampPageLength(c.in); got != c.want {
				t.Fatalf("ClampPageLength(%d) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// The gateways convert page sizes to int32 for the wire. Clamping first is
// what keeps that conversion from wrapping into a negative Limit, which GORM
// reads as "no limit" and turns into a full table read.
func TestClampPageLengthBeforeNarrowingCannotGoNegative(t *testing.T) {
	for _, in := range []int64{math.MaxInt32 + 1, math.MaxInt64, 1 << 40} {
		if got := int32(ClampPageLength(in)); got <= 0 {
			t.Fatalf("int32(ClampPageLength(%d)) = %d, which reads as unbounded", in, got)
		}
	}
}

func TestClampPageAndOffset(t *testing.T) {
	for in, want := range map[int]int{-3: 1, 0: 1, 1: 1, 7: 7} {
		if got := ClampPage(in); got != want {
			t.Fatalf("ClampPage(%d) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[int]int{-3: 0, 0: 0, 42: 42} {
		if got := ClampOffset(in); got != want {
			t.Fatalf("ClampOffset(%d) = %d, want %d", in, got, want)
		}
	}
}
