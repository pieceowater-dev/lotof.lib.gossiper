package gossiper

import "strings"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike escapes the LIKE wildcards (%, _) and the escape character
// itself in user-supplied search text, so a search for "%" matches a literal
// percent sign instead of the whole table. Use it together with
// `ESCAPE '\'`, or see LikeContains.
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}

// LikeContains builds the argument for `col LIKE ? ESCAPE '\'`: the escaped
// search text wrapped in % for a "contains" match.
func LikeContains(s string) string {
	return "%" + EscapeLike(s) + "%"
}
