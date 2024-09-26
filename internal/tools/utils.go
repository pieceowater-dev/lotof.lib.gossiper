package gossiper

type Tools struct{}

func (t *Tools) Split(s string, sep rune) []string {
	var parts []string
	var part []rune
	for _, c := range s {
		if c == sep {
			if len(part) > 0 {
				parts = append(parts, string(part))
				part = nil
			}
		} else {
			part = append(part, c)
		}
	}
	if len(part) > 0 {
		parts = append(parts, string(part))
	}
	return parts
}
