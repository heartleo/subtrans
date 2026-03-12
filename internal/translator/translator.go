package translator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// maxTranslationRetries is the maximum number of retry attempts for missing translations.
const maxTranslationRetries = 3

// collectMissingLines returns the line numbers of lines with no translation yet.
func collectMissingLines(lines []*Line) []int {
	var missing []int
	for _, l := range lines {
		if l.Translation == "" {
			missing = append(missing, l.Number)
		}
	}
	return missing
}

// Translate runs the serial translation loop over all batches.
func Translate(ctx context.Context, batches []*Batch, opts Options, completer Completer, handler TranslationHandler) {
	batchSummary := ""

	for _, batch := range batches {
		if err := ctx.Err(); err != nil {
			handler.OnError(0, fmt.Errorf("context cancelled: %w", err))
			return
		}

		pct := PromptContext{
			BatchSummary: batchSummary,
		}

		messages := BuildPrompt(batch.Lines, pct, opts)
		raw, err := completer.Complete(ctx, messages)

		if err != nil {
			handler.OnError(batch.Number, fmt.Errorf("API error: %w", err))
			return
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

		// Use line.Translation == "" as the authoritative source for what is still missing.
		missingNums := collectMissingLines(batch.Lines)

		for attempt := 1; attempt <= maxTranslationRetries && len(missingNums) > 0; attempt++ {
			slog.Warn("retrying missing lines",
				"batch", batch.Number,
				"attempt", attempt, "missing_count", len(missingNums), "missing_lines", missingNums)

			retryMessages := BuildRetryPrompt(batch.Lines, missingNums, opts)
			retryRaw, retryErr := completer.Complete(ctx, retryMessages)
			if retryErr != nil {
				slog.Error("retry API call failed",
					"batch", batch.Number,
					"attempt", attempt, "error", retryErr)
				break
			}

			retryResult, retryParseErr := ParseResponse(retryRaw, batch.Lines)
			if retryParseErr != nil && !errors.Is(retryParseErr, ErrNoMatches) {
				slog.Error("retry response parse failed",
					"batch", batch.Number,
					"attempt", attempt, "error", retryParseErr)
			} else if retryParseErr == nil {
				result = retryResult
			}

			missingNums = collectMissingLines(batch.Lines)
			slog.Info("retry result",
				"batch", batch.Number,
				"attempt", attempt, "still_missing", len(missingNums))
		}

		if len(missingNums) > 0 {
			err := fmt.Errorf("%d lines still untranslated after %d retries: %v",
				len(missingNums), maxTranslationRetries, missingNums)
			slog.Error("translation incomplete, aborting",
				"batch", batch.Number,
				"missing_count", len(missingNums), "missing_lines", missingNums)
			batch.Errors = append(batch.Errors, err)
			handler.OnError(batch.Number, err)
			return
		}

		batch.Summary = result.BatchSummary
		batchSummary = result.BatchSummary

		slog.Info("batch done",
			"batch", batch.Number, "lines", len(batch.Lines), "summary", result.BatchSummary)

		handler.OnBatchDone(batch.Number, batch.Lines)
	}

	allLines := make([]*Line, 0)
	for _, batch := range batches {
		allLines = append(allLines, batch.Lines...)
	}

	handler.OnDone(allLines)
}
