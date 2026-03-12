package translator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptContext holds contextual information injected into each batch prompt.
type PromptContext struct {
	BatchSummary string // summary from the previous batch
}

// subtitleInput is the JSON structure sent to the LLM for each subtitle line.
type subtitleInput struct {
	Number      int    `json:"number"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
}

const retryContextLines = 5

// BuildPrompt constructs the message slice for an API call.
func BuildPrompt(lines []*Line, ctx PromptContext, opts Options) []Message {
	messages := make([]Message, 0, 3)

	sysContent := buildSystemMessage(opts)
	if sysContent != "" {
		messages = append(messages, Message{Role: "system", Content: sysContent})
	}

	messages = append(messages, Message{Role: "user", Content: buildUserContent(lines, ctx, opts)})

	return messages
}

// BuildRetryPrompt builds a new lightweight request containing only the missing
// lines plus a small context window (up to retryContextLines preceding lines).
// Context lines include their existing translations so the LLM can maintain style
// consistency. This avoids resending the entire batch and saves tokens.
func BuildRetryPrompt(batchLines []*Line, missingNums []int, opts Options) []Message {
	// Build index map: line number -> index in batchLines.
	numToIdx := make(map[int]int, len(batchLines))
	for i, l := range batchLines {
		numToIdx[l.Number] = i
	}

	// Collect indices that need to be included (context + missing), deduplicated.
	include := make(map[int]bool, len(missingNums)*(retryContextLines+1))
	missingSet := make(map[int]bool, len(missingNums))
	for _, num := range missingNums {
		missingSet[num] = true
		idx, ok := numToIdx[num]
		if !ok {
			continue
		}
		// Add context lines before the missing line.
		start := idx - retryContextLines
		if start < 0 {
			start = 0
		}
		for j := start; j <= idx; j++ {
			include[j] = true
		}
	}

	// Build the input slice in order.
	input := make([]subtitleInput, 0, len(include))
	for i, l := range batchLines {
		if !include[i] {
			continue
		}
		entry := subtitleInput{Number: l.Number, Text: l.Text}
		if !missingSet[l.Number] && l.Translation != "" {
			entry.Translation = l.Translation
		}
		input = append(input, entry)
	}

	nums := make([]string, len(missingNums))
	for i, n := range missingNums {
		nums[i] = fmt.Sprintf("%d", n)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Some lines were not translated. Translate ONLY lines: %s\n", strings.Join(nums, ", "))
	b.WriteString("Lines with a \"translation\" field are context only — do NOT re-translate them.\n\n")
	jsonBytes, _ := json.Marshal(input)
	b.Write(jsonBytes)

	messages := make([]Message, 0, 2)
	messages = append(messages, Message{Role: "system", Content: buildSystemMessage(opts)})
	messages = append(messages, Message{Role: "user", Content: b.String()})

	return messages
}

func buildSystemMessage(opts Options) string {
	var b strings.Builder

	if opts.TargetLanguage != "" {
		langName := ResolveLanguage(opts.TargetLanguage)
		_, _ = fmt.Fprintf(&b, "You are a professional subtitle translator. Translate the subtitles into %s.\n", langName)
		b.WriteString("Preserve the original meaning, tone, and style. ")
		b.WriteString("Use natural, fluent expressions in the target language. ")
		b.WriteString("Do not add or remove content.\n\n")
		b.WriteString("You MUST respond in the following JSON format:\n")
		b.WriteString("{\n")
		b.WriteString(`  "translations": [`)
		b.WriteString("\n")
		b.WriteString(`    {"number": 1, "translation": "translated text here"},`)
		b.WriteString("\n")
		b.WriteString(`    {"number": 2, "translation": "translated text here"}`)
		b.WriteString("\n")
		b.WriteString("  ],\n")
		b.WriteString(`  "batch_summary": "brief summary of this batch"`)
		b.WriteString("\n}\n\n")
		b.WriteString("Rules:\n")
		b.WriteString("- Output valid JSON only. No markdown, no extra text.\n")
		b.WriteString("- The number of output translations MUST equal the number of input lines. No more, no less.\n")
		b.WriteString("- Every input line MUST have a corresponding translation. Do NOT skip or merge any lines.\n")
		b.WriteString("- The \"number\" field must match the input subtitle number exactly.\n")
		b.WriteString("- The \"batch_summary\" should briefly describe the content of this batch.\n\n")
	}

	if opts.Instructions != "" {
		b.WriteString(opts.Instructions)
	}

	return strings.TrimSpace(b.String())
}

func buildUserContent(lines []*Line, ctx PromptContext, opts Options) string {
	var b strings.Builder

	if ctx.BatchSummary != "" {
		_, _ = fmt.Fprintf(&b, "Previous batch summary: %s\n\n", ctx.BatchSummary)
	}

	if opts.Prompt != "" {
		_, _ = fmt.Fprintf(&b, "%s\n\n", opts.Prompt)
	}

	input := make([]subtitleInput, len(lines))
	for i, line := range lines {
		input[i] = subtitleInput{Number: line.Number, Text: line.Text}
	}

	jsonBytes, _ := json.Marshal(input)
	b.Write(jsonBytes)

	return b.String()
}
