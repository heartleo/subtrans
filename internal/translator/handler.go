package translator

import "context"

// Completer is the interface for calling the LLM API.
type Completer interface {
	Complete(ctx context.Context, messages []Message) (string, error)
}

// TranslationHandler receives translation progress callbacks.
// OnError indicates the translation has failed and OnDone will not be called.
type TranslationHandler interface {
	OnBatchDone(batch int, lines []*Line)
	OnError(batch int, err error)
	OnDone(lines []*Line)
}

// BaseHandler provides no-op implementations of TranslationHandler.
// Embed it to only override the methods you care about.
type BaseHandler struct{}

func (BaseHandler) OnBatchDone(int, []*Line) {}
func (BaseHandler) OnError(int, error)       {}
func (BaseHandler) OnDone([]*Line)           {}
