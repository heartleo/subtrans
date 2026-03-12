// Package api provides the HTTP handler and SSE writer for subtrans.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/heartleo/subtrans/internal/srt"
	"github.com/heartleo/subtrans/internal/translator"
)

type sseLineJSON struct {
	Number      int    `json:"number"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Translation string `json:"translation"`
}

type ssePayload struct {
	Event   string        `json:"event"`
	Batch   int           `json:"batch,omitempty"`
	Lines   []sseLineJSON `json:"lines,omitempty"`
	SRT     string        `json:"srt,omitempty"`
	Message string        `json:"message,omitempty"`
}

// sseHandler implements translator.TranslationHandler for SSE streaming.
type sseHandler struct {
	w       http.ResponseWriter
	fmtOpts srt.FormatOptions
}

func (h *sseHandler) OnBatchDone(batch int, lines []*translator.Line) {
	payload := ssePayload{
		Event: "batch_done",
		Batch: batch,
		Lines: make([]sseLineJSON, 0, len(lines)),
	}

	for _, l := range lines {
		if l.Translation == "" {
			slog.Warn("line with empty translation in batch done event", "line", l.Number)
			continue
		}
		payload.Lines = append(payload.Lines, sseLineJSON{
			Number:      l.Number,
			Start:       srt.FormatTimestamp(l.Start),
			End:         srt.FormatTimestamp(l.End),
			Translation: l.Translation,
		})
	}

	h.send(payload)
}

func (h *sseHandler) OnError(batch int, err error) {
	payload := ssePayload{
		Event:   "error",
		Batch:   batch,
		Message: err.Error(),
	}
	h.send(payload)
}

func (h *sseHandler) OnDone(lines []*translator.Line) {
	payload := ssePayload{
		Event: "done",
		SRT:   srt.Format(lines, h.fmtOpts),
	}
	h.send(payload)
}

func (h *sseHandler) send(payload ssePayload) {
	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal SSE event", "error", err)
		return
	}

	fmt.Fprintf(h.w, "data: %s\n\n", b)

	if f, ok := h.w.(http.Flusher); ok {
		f.Flush()
	}
}
