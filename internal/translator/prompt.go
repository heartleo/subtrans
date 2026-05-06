package translator

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	retryContextLines = 5
	batchContextLines = 3
)

// mustMarshal marshals v to JSON. Panics if v contains unmarshalable types
// (channels, funcs). Safe for structs with only basic field types.
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("BUG: json.Marshal failed for %T: %v", v, err))
	}
	return b
}

// PromptContext holds contextual information injected into each batch prompt.
type PromptContext struct {
	ContextLines []*Line // last N translated lines from the previous batch
}

// subtitleInput is the JSON structure sent to the LLM for each subtitle line.
type subtitleInput struct {
	Number      int    `json:"number"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
}

// BuildPrompt constructs the message slice for an initial batch API call.
// Lines are sent with sequential 1-based indices so the LLM is not confused
// by large or non-sequential SRT line numbers.
func BuildPrompt(lines []*Line, ctx PromptContext, opts Options) []Message {
	messages := make([]Message, 0, 3)

	if sys := buildSystemMessage(opts); sys != "" {
		messages = append(messages, Message{Role: "system", Content: sys})
	}
	messages = append(messages, Message{Role: "user", Content: buildUserContent(lines, ctx, opts)})
	return messages
}

// BuildRetryPrompt builds a lightweight request for missing lines plus a small
// context window of already-translated lines. Returns the messages and the
// ordered slice of lines that was sent — callers must pass that slice to
// ParseResponse so sequential indices can be resolved correctly.
func BuildRetryPrompt(batchLines []*Line, missingNums []int, opts Options) ([]Message, []*Line) {
	numToIdx := make(map[int]int, len(batchLines))
	for i, l := range batchLines {
		numToIdx[l.Number] = i
	}

	include := make(map[int]bool, len(missingNums)*(retryContextLines+1))
	missingSet := make(map[int]bool, len(missingNums))
	for _, num := range missingNums {
		missingSet[num] = true
		idx, ok := numToIdx[num]
		if !ok {
			continue
		}
		for j := max(idx-retryContextLines, 0); j <= idx; j++ {
			include[j] = true
		}
	}

	// Collect lines in order, assign sequential indices.
	var retryLines []*Line
	for i, l := range batchLines {
		if include[i] {
			retryLines = append(retryLines, l)
		}
	}

	input := make([]subtitleInput, len(retryLines))
	var missingSeqNums []string
	for i, l := range retryLines {
		seqNum := i + 1
		entry := subtitleInput{Number: seqNum, Text: l.Text}
		if !missingSet[l.Number] && l.Translation != "" {
			entry.Translation = l.Translation // context-only line
		} else {
			missingSeqNums = append(missingSeqNums, fmt.Sprintf("%d", seqNum))
		}
		input[i] = entry
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Some lines were not translated. Translate ONLY lines numbered: %s\n", strings.Join(missingSeqNums, ", "))
	b.WriteString("Lines that already have a \"translation\" field are context only — do NOT re-translate them.\n\n")
	b.Write(mustMarshal(input))

	messages := []Message{
		{Role: "system", Content: buildSystemMessage(opts)},
		{Role: "user", Content: b.String()},
	}
	return messages, retryLines
}

func buildSystemMessage(opts Options) string {
	var b strings.Builder

	if opts.TargetLanguage != "" {
		langName := ResolveLanguage(opts.TargetLanguage)
		fmt.Fprintf(&b, "You are a professional subtitle translator. Translate the subtitles into %s.\n", langName)
		b.WriteString("Preserve the original meaning, tone, and style. ")
		b.WriteString("Use natural, fluent expressions in the target language. ")
		b.WriteString("Do not add or remove content.\n\n")
		b.WriteString("You MUST respond in the following JSON format:\n")
		b.WriteString("{\n")
		b.WriteString("  \"translations\": [\n")
		b.WriteString("    {\"number\": 1, \"translation\": \"translated text here\"},\n")
		b.WriteString("    {\"number\": 2, \"translation\": \"translated text here\"}\n")
		b.WriteString("  ]\n")
		b.WriteString("}\n\n")
		b.WriteString("Rules:\n")
		b.WriteString("- Output valid JSON only. No markdown, no extra text.\n")
		b.WriteString("- Translate EVERY input line. The output array length MUST equal the input array length.\n")
		b.WriteString("- Each line must be translated as a standalone unit. Do NOT merge consecutive lines or split one line into multiple translations, even if the line appears to be an incomplete sentence.\n")
		b.WriteString("- The \"number\" field must match the input number exactly.\n")
	}

	if opts.Instructions != "" {
		b.WriteString(opts.Instructions)
	}

	return strings.TrimSpace(b.String())
}

func buildUserContent(lines []*Line, ctx PromptContext, opts Options) string {
	var b strings.Builder

	if len(ctx.ContextLines) > 0 {
		b.WriteString("Context from previous batch (do not retranslate):\n")
		ctxInput := make([]subtitleInput, len(ctx.ContextLines))
		for i, l := range ctx.ContextLines {
			ctxInput[i] = subtitleInput{Number: i + 1, Text: l.Text, Translation: l.Translation}
		}
		b.Write(mustMarshal(ctxInput))
		b.WriteString("\n\n")
	}

	if opts.Prompt != "" {
		fmt.Fprintf(&b, "%s\n\n", opts.Prompt)
	}

	// Sequential 1-based indices within this batch.
	input := make([]subtitleInput, len(lines))
	for i, line := range lines {
		input[i] = subtitleInput{Number: i + 1, Text: line.Text}
	}
	b.Write(mustMarshal(input))

	return b.String()
}
