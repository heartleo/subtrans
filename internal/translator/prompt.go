package translator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	retryContextLines = 5
	batchContextLines = 3
)

const (
	systemIntroTmpl  = "You are a professional subtitle translator. Translate the subtitles into %s.\n"
	systemStyleRules = `Preserve the original meaning, tone, and style. Use natural, fluent expressions in the target language. Do not add or remove content.` + "\n\n"
	systemJSONFormat = `You MUST respond in the following JSON format:
{
  "translations": [
    {"number": 1, "translation": "translated text here"},
    {"number": 2, "translation": "translated text here"}
  ]
}` + "\n\n"
	systemRules = `Rules:
- Output valid JSON only. No markdown, no extra text.
- Translate EVERY input line. The output array length MUST equal the input array length.
- Each line must be translated as a standalone unit. Do NOT merge consecutive lines or split one line into multiple translations, even if the line appears to be an incomplete sentence.
- If a sentence spans multiple lines, split the translation at the SAME boundary as the source. Line N translation must contain ONLY the meaning of source line N.
- NEVER leave a translation empty by moving its content into an adjacent line.
- Example of WRONG behavior (content of line 2 merged into line 1):
  Input:  [{"number":1,"text":"For X, they simplify the button,"},{"number":2,"text":"by removing Y, saving costs."}]
  WRONG:  [{"number":1,"translation":"对X来说,他们通过移除Y简化按钮,节省成本"},{"number":2,"translation":""}]
  RIGHT:  [{"number":1,"translation":"对X来说,他们简化了按钮,"},{"number":2,"translation":"通过移除Y,节省了成本。"}]
- The "number" field must match the input number exactly.` + "\n"
	retryInstructionTmpl = "Some lines were not translated. Translate ONLY lines numbered: %s\n"
	retryContextNote     = `Lines that already have a "translation" field are context only — do NOT re-translate them.` + "\n\n"
	batchContextHeader   = "Context from previous batch (do not retranslate):\n"
)

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
			missingSeqNums = append(missingSeqNums, strconv.Itoa(seqNum))
		}
		input[i] = entry
	}

	var b strings.Builder
	fmt.Fprintf(&b, retryInstructionTmpl, strings.Join(missingSeqNums, ", "))
	b.WriteString(retryContextNote)
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
		fmt.Fprintf(&b, systemIntroTmpl, langName)
		b.WriteString(systemStyleRules)
		b.WriteString(systemJSONFormat)
		b.WriteString(systemRules)
	}

	if opts.Instructions != "" {
		b.WriteString(opts.Instructions)
	}

	return strings.TrimSpace(b.String())
}

func buildUserContent(lines []*Line, ctx PromptContext, opts Options) string {
	var b strings.Builder

	if len(ctx.ContextLines) > 0 {
		b.WriteString(batchContextHeader)
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
