package translator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// maxTranslationRetries is the maximum number of retry attempts for missing translations.
const maxTranslationRetries = 3

// collectMissingLines returns the original SRT line numbers of lines with no translation yet.
func collectMissingLines(lines []*Line) []int {
	var missing []int
	for _, l := range lines {
		if l.Translation == "" {
			missing = append(missing, l.Number)
		}
	}
	return missing
}

// lastNLines returns the last n lines of the slice, or the whole slice if len ≤ n.
func lastNLines(lines []*Line, n int) []*Line {
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// Translate runs the serial translation loop over all batches.
// Returns the first fatal error encountered; handler still receives progress
// callbacks (OnBatchDone, OnError). OnDone fires only on full success.
func Translate(ctx context.Context, batches []*Batch, opts Options, completer Completer, handler TranslationHandler) error {
	var contextLines []*Line

	for _, batch := range batches {
		if err := ctx.Err(); err != nil {
			wrapped := fmt.Errorf("context cancelled: %w", err)
			handler.OnError(0, wrapped)
			return wrapped
		}

		messages := BuildPrompt(batch.Lines, PromptContext{ContextLines: contextLines}, opts)
		raw, err := completer.Complete(ctx, messages)
		if err != nil {
			wrapped := fmt.Errorf("batch %d: API error: %w", batch.Number, err)
			handler.OnError(batch.Number, wrapped)
			return wrapped
		}

		result, parseErr := ParseResponse(raw, batch.Lines)
		batch.RawResponse = raw

		switch {
		case errors.Is(parseErr, ErrNoMatches):
			slog.Warn("no translation entries found in response, all lines treated as missing",
				"batch", batch.Number, "lines", len(batch.Lines))
		case parseErr != nil:
			slog.Error("unexpected parse error",
				"batch", batch.Number, "error", parseErr)
		default:
			slog.Info("initial parse complete",
				"batch", batch.Number,
				"translated", len(batch.Lines)-len(result.Missing), "missing", len(result.Missing))
		}

		missingNums := collectMissingLines(batch.Lines)

		for attempt := 1; attempt <= maxTranslationRetries && len(missingNums) > 0; attempt++ {
			slog.Warn("retrying missing lines",
				"batch", batch.Number,
				"attempt", attempt, "missing_count", len(missingNums), "missing_lines", missingNums)

			retryMessages, retryLines := BuildRetryPrompt(batch.Lines, missingNums, opts)
			retryRaw, retryErr := completer.Complete(ctx, retryMessages)
			if retryErr != nil {
				slog.Error("retry API call failed",
					"batch", batch.Number,
					"attempt", attempt, "error", retryErr)
				break
			}

			_, retryParseErr := ParseResponse(retryRaw, retryLines)
			if retryParseErr != nil && !errors.Is(retryParseErr, ErrNoMatches) {
				slog.Error("retry response parse failed",
					"batch", batch.Number,
					"attempt", attempt, "error", retryParseErr)
			}

			missingNums = collectMissingLines(batch.Lines)
			slog.Info("retry result",
				"batch", batch.Number,
				"attempt", attempt, "still_missing", len(missingNums))
		}

		if len(missingNums) > 0 {
			fatal := fmt.Errorf("batch %d: %d lines still untranslated after %d retries: %v",
				batch.Number, len(missingNums), maxTranslationRetries, missingNums)
			slog.Error("translation incomplete, aborting",
				"batch", batch.Number,
				"missing_count", len(missingNums), "missing_lines", missingNums)
			batch.Errors = append(batch.Errors, fatal)
			handler.OnError(batch.Number, fatal)
			return fatal
		}

		contextLines = lastNLines(batch.Lines, batchContextLines)

		slog.Info("batch done", "batch", batch.Number, "lines", len(batch.Lines))
		handler.OnBatchDone(batch.Number, batch.Lines)
	}

	var allLines []*Line
	for _, batch := range batches {
		allLines = append(allLines, batch.Lines...)
	}
	handler.OnDone(allLines)
	return nil
}
