package translator

import "strings"

// BatchLines divides lines into batches of approximately opts.MaxBatchSize.
// A batch boundary is placed after a line whose text ends with one of the
// characters in opts.BatchSplitPunctuation, so that each batch contains complete
// sentences. If no matching punctuation is found the batch is extended until
// the next match or the end of input.
func BatchLines(lines []*Line, opts Options) []*Batch {
	if len(lines) == 0 {
		return nil
	}

	maxSize := opts.MaxBatchSize
	if maxSize <= 0 {
		maxSize = 30
	}

	p := opts.BatchSplitPunctuation
	if p == "" {
		p = "."
	}

	var batches []*Batch
	start := 0

	for i, line := range lines {
		size := i - start + 1
		if size >= maxSize && endsWithPunctuation(line.Text, p) {
			batches = append(batches, &Batch{Lines: lines[start : i+1]})
			start = i + 1
		}
	}

	// Remaining lines form the last batch.
	if start < len(lines) {
		batches = append(batches, &Batch{Lines: lines[start:]})
	}

	for i, b := range batches {
		b.Number = i + 1
	}

	return batches
}

// endsWithPunctuation reports whether text ends with any character in punctuation.
func endsWithPunctuation(text string, p string) bool {
	t := strings.TrimRight(text, " \t\r\n")
	if t == "" {
		return false
	}
	lastChar := t[len(t)-1:]
	return strings.ContainsAny(lastChar, p)
}
