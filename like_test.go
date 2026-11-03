package gossiper

import "testing"

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"abc": "abc",
		"50%": `50\%`,
		"a_b": `a\_b`,
		`a\b`: `a\\b`,
		`%_\`: `\%\_\\`,
		"":    "",
	}
	for in, want := range cases {
		if got := EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q)=%q want %q", in, got, want)
		}
	}
	if got := LikeContains("a%"); got != `%a\%%` {
		t.Errorf("LikeContains=%q", got)
	}
}
