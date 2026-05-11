package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/heartleo/subtrans/internal/config"
	"github.com/heartleo/subtrans/internal/subtitle"
	_ "github.com/heartleo/subtrans/internal/subtitle/all" // register codecs
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
		language = translator.DefaultOptions().TargetLanguage
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "file is required")
		return
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Error("failed to close file", "error", closeErr)
		}
	}()

	contentBytes, readErr := io.ReadAll(file)
	if readErr != nil {
		writeJSON(w, http.StatusBadRequest, "failed to read file: "+readErr.Error())
		return
	}
	content := string(contentBytes)

	format := subtitle.FormatSRT
	if formStr := r.FormValue("format"); formStr != "" {
		format = subtitle.Format(formStr)
	} else if fileHeader != nil {
		if f, err := subtitle.DetectByExt(fileHeader.Filename); err == nil {
			format = f
		}
	}
	codec, err := subtitle.For(format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	doc, parseErr := codec.Parse(content)
	if parseErr != nil {
		writeJSON(w, http.StatusBadRequest, "invalid subtitle: "+parseErr.Error())
		return
	}

	opts := translator.DefaultOptions()
	opts.TargetLanguage = language
	opts.Prompt = r.FormValue("prompt")
	opts.Instructions = r.FormValue("instructions")

	batches := translator.BatchLines(doc.Lines, opts)
	fmtOpts := subtitle.FormatOptions{
		IncludeOriginal:          opts.IncludeOriginal,
		StripTrailingPunctuation: opts.StripTrailingPunctuation,
	}

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		h.serveSSE(w, r, batches, opts, codec, doc, fmtOpts)
	} else {
		h.servePlain(w, r, batches, opts, codec, doc, fmtOpts)
	}
}

// servePlain runs translation synchronously and returns subtitle text directly.
func (h *Handler) servePlain(w http.ResponseWriter, r *http.Request, batches []*translator.Batch, opts translator.Options, codec subtitle.Codec, doc *subtitle.Document, fmtOpts subtitle.FormatOptions) {
	if err := translator.Translate(r.Context(), batches, opts, h.completer, translator.BaseHandler{}); err != nil {
		writeJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(codec.Format(doc, fmtOpts))) // #nosec G705 -- text/plain subtitle output, not HTML
}

// serveSSE streams translation progress as Server-Sent Events.
func (h *Handler) serveSSE(w http.ResponseWriter, r *http.Request, batches []*translator.Batch, opts translator.Options, codec subtitle.Codec, doc *subtitle.Document, fmtOpts subtitle.FormatOptions) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sh := &sseHandler{w: w, codec: codec, doc: doc, fmtOpts: fmtOpts}
	// Errors are streamed to the client via sseHandler.OnError.
	_ = translator.Translate(r.Context(), batches, opts, h.completer, sh)
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
