# subtrans

***Translate subtitle files (SRT / VTT / ASS / LRC / SBV) using OpenAI API***

![Go Version](https://img.shields.io/badge/go-1.25%2B-blue)
[![Go Report Card](https://goreportcard.com/badge/github.com/heartleo/subtrans)](https://goreportcard.com/report/github.com/heartleo/subtrans)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/subtrans/release.yml)](https://github.com/heartleo/subtrans/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/subtrans)](https://github.com/heartleo/subtrans/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/subtrans/total)](https://github.com/heartleo/subtrans/releases)
![License](https://img.shields.io/badge/license-MIT-green)

[中文](README.md) | [English](README_en.md)

## Features

- Multiple subtitle formats: **SRT**, **WebVTT**, **ASS/SSA**, **LRC**, **SBV**
- Works with any OpenAI-compatible API
- Smart batch splitting by sentence boundaries
- Automatic retry for missing or merged translations
- Customizable translation instructions and prompts
- Bilingual output (original + translated text)
- Use as a **CLI tool** or **Go library**
- HTTP API with SSE streaming support

## Installation

**Homebrew** (macOS / Linux):

```bash
brew install heartleo/tap/subtrans
```

<!-- **winget** (Windows):

```powershell
winget install heartleo.subtrans
```
-->

**curl** (macOS / Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/heartleo/subtrans/main/install.sh | sh
```

**Go install** (requires Go 1.25+):

```bash
go install github.com/heartleo/subtrans/cmd/subtrans@latest
```

**Build from source:**

```bash
git clone https://github.com/heartleo/subtrans
cd subtrans
go build -o subtrans ./cmd/subtrans
```

## Quick Start

### CLI

```bash
# Create env file or set environment variables
cat > .env <<EOF
OPENAI_API_KEY=sk-xxx
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4.1
EOF

# Translate to Chinese (default; format auto-detected by extension)
subtrans input.srt
subtrans input.vtt
subtrans lyrics.lrc
subtrans anime.ass

# Translate to French with custom output path
subtrans -l fr -o output.fr.srt input.srt

# Use a custom API base URL and model
export OPENAI_BASE_URL=https://your-api.com/v1
export OPENAI_MODEL=gpt-5.2
subtrans -l fr input.srt
```

### Go Library

```bash
go get github.com/heartleo/subtrans
```

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/heartleo/subtrans"
)

func main() {
	t, err := subtrans.New(subtrans.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}

	subContent, err := os.ReadFile("input.srt")
	if err != nil {
		log.Fatal(err)
	}

	result, err := t.Translate(context.TODO(), string(subContent), "zh")
	if err != nil {
		log.Fatal(err)
	}
}
```

For non-SRT formats, pass `WithFormat`:

```go
t.Translate(ctx, content, "zh", subtrans.WithFormat(subtrans.FormatVTT))
t.Translate(ctx, content, "zh", subtrans.WithFormat(subtrans.FormatASS))

// Or detect by filename
f, _ := subtrans.DetectFormat("input.lrc")
t.Translate(ctx, content, "zh", subtrans.WithFormat(f))
```

## Supported Formats

| Format | Extension     | Supported |
| ------ | ------------- | --------- |
| SRT    | `.srt`        | ✅         |
| VTT    | `.vtt`        | ✅         |
| ASS    | `.ass`/`.ssa` | ✅         |
| LRC    | `.lrc`        | ✅         |
| SBV    | `.sbv`        | ✅         |

## Environment Variables

| Variable             | Description  | Default                     |
| -------------------- | ------------ | --------------------------- |
| `OPENAI_API_KEY`     | API key      | -                           |
| `OPENAI_BASE_URL`    | API base URL | `https://api.openai.com/v1` |
| `OPENAI_MODEL`       | Model name   | `gpt-5.5`                   |
| `OPENAI_TEMPERATURE` | Temperature  | `0.0`                       |
| `OPENAI_MAX_RETRIES` | Max retries  | `3`                         |

## Server API

```bash
# Start server
subtrans serve

# Custom prompt
curl -X POST http://localhost:8091/translate \
  -F "file=@input.srt" \
  -F "language=fr" \
  -F "prompt=your-prompt"

# Upload a VTT/ASS/LRC/SBV file
curl -X POST http://localhost:8091/translate \
  -F "file=@anime.ass" \
  -F "language=zh"

# Specify format explicitly
curl -X POST http://localhost:8091/translate \
  -F "file=@noext_file" \
  -F "format=vtt" \
  -F "language=zh"

# SSE streaming, returns results batch by batch
curl -X POST http://localhost:8091/translate \
  -H "Accept: text/event-stream" \
  -F "file=@input.srt" \
  -F "language=zh"
```

| Parameter      | Description                                     | Default      |
| -------------- | ----------------------------------------------- | ------------ |
| `file`         | Subtitle file                                   | -            |
| `format`       | Subtitle format (`srt`/`vtt`/`ass`/`lrc`/`sbv`) | by extension |
| `language`     | Target language ISO code                        | `zh`         |
| `prompt`       | Custom user prompt                              | -            |
| `instructions` | Custom system instructions                      | -            |

## CLI Flags

Global flags (available on all subcommands):

| Flag        | Short | Description          | Default |
| ----------- | ----- | -------------------- | ------- |
| `--verbose` | `-v`  | Enable debug logging | `false` |

### subtrans

| Flag                  | Short | Description                                                   | Default                |
| --------------------- | ----- | ------------------------------------------------------------- | ---------------------- |
| `--language`          | `-l`  | Target language ISO code                                      | `zh`                   |
| `--output`            | `-o`  | Output file path                                              | `<input>.<lang>.<ext>` |
| `--model`             | `-m`  | Model                                                         | -                      |
| `--max-batch-size`    |       | Lines per batch                                               | `30`                   |
| `--batch-split-punct` |       | Punctuation for splitting                                     | `.`                    |
| `--instructions`      |       | Path to instructions file                                     | -                      |
| `--prompt`            |       | Custom user prompt                                            | -                      |
| `--temperature`       |       | Temperature                                                   | `0.0`                  |
| `--max-retries`       |       | API retry count                                               | `3`                    |
| `--include-original`  |       | Include original text in output                               | `false`                |
| `--strip-punctuation` |       | Strip trailing punctuation from translation and original text | `true`                 |

### subtrans serve

| Flag     | Short | Description | Default     |
| -------- | ----- | ----------- | ----------- |
| `--host` |       | Listen host | `localhost` |
| `--port` | `-p`  | Listen port | `8091`      |

---

<div align="center">

Made with ❤️ by [heartleo](https://github.com/heartleo)

</div>
