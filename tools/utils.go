package tools

func Split(s string, sep rune) []string {
	var parts []string
	var part string
	for _, c := range s {
		if c == sep {
			parts = append(parts, part)
			part = ""
		} else {
			part += string(c)
		}
	}
	if part != "" {
		parts = append(parts, part)
	}
	return parts
}
