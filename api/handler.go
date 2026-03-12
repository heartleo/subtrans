package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/srt"
	"github.com/heartleo/subtrans/internal/translator"
)

// Handler is the HTTP handler for the /translate endpoint.
type Handler struct {
	cfg       config.Config
	completer translator.Completer
}

// NewHandler creates a Handler with the given config and Completer.
func NewHandler(cfg config.Config, completer translator.Completer) *Handler {
	return &Handler{cfg: cfg, completer: completer}
}

// ServeHTTP handles POST /translate.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	language := r.FormValue("language")
	if language == "" {
		language = "zh"
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "file is required")
		return
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Error("failed to close file", "error", closeErr)
		}
	}()

	buf := make([]byte, 10<<20)
	n, readErr := file.Read(buf)
	if readErr != nil && n == 0 {
		writeJSON(w, http.StatusBadRequest, "failed to read file: "+readErr.Error())
		return
	}
	srtContent := string(buf[:n])

	lines, parseErr := srt.Parse(srtContent)
	if parseErr != nil {
		writeJSON(w, http.StatusBadRequest, "invalid SRT: "+parseErr.Error())
		return
	}

	opts := translator.DefaultOptions()
	opts.TargetLanguage = language
	opts.Prompt = r.FormValue("prompt")
	opts.Instructions = r.FormValue("instructions")

	batches := translator.BatchLines(lines, opts)
	fmtOpts := srt.FormatOptions{
		IncludeOriginal:          opts.IncludeOriginal,
		StripTrailingPunctuation: opts.StripTrailingPunctuation,
	}

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		h.serveSSE(w, r, batches, opts, fmtOpts)
	} else {
		h.servePlain(w, r, batches, opts, fmtOpts)
	}
}

// servePlain runs translation synchronously and returns the SRT text directly.
func (h *Handler) servePlain(w http.ResponseWriter, r *http.Request, batches []*translator.Batch, opts translator.Options, fmtOpts srt.FormatOptions) {
	ph := &plainHandler{}

	translator.Translate(r.Context(), batches, opts, h.completer, ph)

	if ph.Err != "" {
		writeJSON(w, http.StatusInternalServerError, ph.Err)
		return
	}

	allLines := collectLines(batches)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(srt.Format(allLines, fmtOpts)))
}

// serveSSE streams translation progress as Server-Sent Events.
func (h *Handler) serveSSE(w http.ResponseWriter, r *http.Request, batches []*translator.Batch, opts translator.Options, fmtOpts srt.FormatOptions) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sh := &sseHandler{w: w, fmtOpts: fmtOpts}
	translator.Translate(r.Context(), batches, opts, h.completer, sh)
}

func collectLines(batches []*translator.Batch) []*translator.Line {
	var lines []*translator.Line
	for _, batch := range batches {
		lines = append(lines, batch.Lines...)
	}
	return lines
}

func writeJSON(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	payload := map[string]string{"error": msg}

	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal error response", "error", err)
		return
	}

	_, _ = w.Write(b)
}

// plainHandler collects errors for synchronous HTTP responses.
type plainHandler struct {
	translator.BaseHandler
	Err string
}

func (h *plainHandler) OnError(_ int, err error) {
	h.Err = err.Error()
}
