package subtitle

import "strings"

// StripTrailing removes trailing periods and commas (both ASCII and CJK) from
// each line of s. Preserves ellipsis (..., …) and other meaningful
// punctuation (! ? etc). Exposed for codec implementations.
func StripTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = stripTrailingLine(line)
	}
	return strings.Join(lines, "\n")
}

func stripTrailingLine(s string) string {
	s = strings.TrimRight(s, " ")
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "...") || strings.HasSuffix(s, "…") {
		return s
	}
	const trailing = ".," + "，" + "。" + "．"
	return strings.TrimRight(s, trailing)
}
