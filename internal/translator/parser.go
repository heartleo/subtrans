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
	Missing []int // original SRT line numbers that received no translation
}

// translationResponse is the expected JSON structure from the LLM.
type translationResponse struct {
	Translations []translationEntry `json:"translations"`
}

type translationEntry struct {
	Number      int    `json:"number"`
	Translation string `json:"translation"`
}

// ParseResponse extracts translations from the raw AI response and writes them
// into the corresponding Line pointers. lines must be the same ordered slice
// that was passed to BuildPrompt or BuildRetryPrompt — matching is by sequential
// position (entry number 1 → lines[0], number 2 → lines[1], …).
func ParseResponse(raw string, lines []*Line) (ParseResult, error) {
	cleaned := stripCodeFence(raw)

	var resp translationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		slog.Error("failed to parse JSON response", "error", err, "raw", raw)
		return ParseResult{}, fmt.Errorf("%w: %v", ErrNoMatches, err)
	}

	if len(resp.Translations) == 0 {
		slog.Warn("JSON parsed but translations array is empty")
		return ParseResult{}, ErrNoMatches
	}

	// seq number (1-based) → translation text
	seqMap := make(map[int]string, len(resp.Translations))
	for _, e := range resp.Translations {
		seqMap[e.Number] = e.Translation
	}

	var missing []int
	for i, line := range lines {
		seqNum := i + 1
		t, ok := seqMap[seqNum]
		if !ok {
			missing = append(missing, line.Number)
			continue
		}
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			slog.Warn("empty translation for line", "line", line.Number)
			missing = append(missing, line.Number)
		} else {
			line.Translation = trimmed
		}
	}

	return ParseResult{Missing: missing}, nil
}

// stripCodeFence removes markdown code fences (```json ... ``` or ``` ... ```)
// from LLM responses, returning the inner content.
func stripCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return raw
	}

	_, inner, ok := strings.Cut(trimmed, "\n")
	if !ok {
		return raw
	}

	if idx := strings.LastIndex(inner, "```"); idx >= 0 {
		inner = inner[:idx]
	}

	return strings.TrimSpace(inner)
}
