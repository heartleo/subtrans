package translator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ErrNoMatches is returned when no translation entries can be extracted.
var ErrNoMatches = errors.New("no translation entries found in response")

// ParseResult holds the output of parsing an AI translation response.
type ParseResult struct {
	BatchSummary string
	Missing      []int // line numbers not found in the response
}

// translationResponse is the expected JSON structure from the LLM.
type translationResponse struct {
	Translations []translationEntry `json:"translations"`
	BatchSummary string             `json:"batch_summary"`
}

type translationEntry struct {
	Number      int    `json:"number"`
	Translation string `json:"translation"`
}

// ParseResponse extracts translations from the AI raw response and writes them
// into the corresponding Line pointers. Returns metadata and missing line numbers.
func ParseResponse(raw string, originals []*Line) (ParseResult, error) {
	cleaned := stripCodeFence(raw)

	var resp translationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		slog.Error("failed to parse JSON response", "error", err, "raw", raw)
		return ParseResult{}, fmt.Errorf("%w: %v", ErrNoMatches, err)
	}

	if len(resp.Translations) == 0 {
		slog.Warn("JSON parsed but translations array is empty")
		return ParseResult{}, fmt.Errorf("%w", ErrNoMatches)
	}

	translationMap := make(map[int]string, len(resp.Translations))
	for _, e := range resp.Translations {
		translationMap[e.Number] = e.Translation
	}

	var missing []int
	for _, line := range originals {
		if t, ok := translationMap[line.Number]; ok {
			trimmed := strings.TrimSpace(t)
			if trimmed == "" {
				slog.Warn("translation is empty string for line", "line", line.Number)
				missing = append(missing, line.Number)
			} else {
				line.Translation = trimmed
			}
		} else {
			missing = append(missing, line.Number)
		}
	}

	return ParseResult{
		BatchSummary: strings.TrimSpace(resp.BatchSummary),
		Missing:      missing,
	}, nil
}

// stripCodeFence removes markdown code fences (```json ... ``` or ``` ... ```)
// from LLM responses, returning the inner content.
func stripCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return raw
	}

	// Remove opening fence line (```json or ```)
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline < 0 {
		return raw
	}
	inner := trimmed[firstNewline+1:]

	// Remove closing fence
	if idx := strings.LastIndex(inner, "```"); idx >= 0 {
		inner = inner[:idx]
	}

	return strings.TrimSpace(inner)
}
