# subtrans

***Translate SRT subtitles using OpenAI API***

![Go Version](https://img.shields.io/badge/go-1.25%2B-blue)
[![Go Report Card](https://goreportcard.com/badge/github.com/heartleo/subtrans)](https://goreportcard.com/report/github.com/heartleo/subtrans)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/subtrans/release.yml)](https://github.com/heartleo/subtrans/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/subtrans)](https://github.com/heartleo/subtrans/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/subtrans/total)](https://github.com/heartleo/subtrans/releases)
![License](https://img.shields.io/badge/license-MIT-green)

[中文](README.md) | [English](README_en.md)

## Features

- Translate `.srt` subtitle files
- Works with any OpenAI-compatible API
- Smart batch splitting by sentence boundaries
- Automatic retry for missing translations
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

# Translate to Chinese (default)
subtrans input.srt

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

	srtContent, err := os.ReadFile("input.srt")
	if err != nil {
		log.Fatal(err)
	}

	result, err := t.Translate(context.TODO(), string(srtContent), "zh")
	if err != nil {
		log.Fatal(err)
	}
}
```

## Environment Variables

| Variable             | Description  | Default                     |
| -------------------- | ------------ | --------------------------- |
| `OPENAI_API_KEY`     | API key      | -                           |
| `OPENAI_BASE_URL`    | API base URL | `https://api.openai.com/v1` |
| `OPENAI_MODEL`       | Model name   | `gpt-4.1`                   |
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

# SSE streaming, returns results batch by batch
curl -X POST http://localhost:8091/translate \
  -H "Accept: text/event-stream" \
  -F "file=@input.srt" \
  -F "language=zh"
```

| Parameter      | Description                | Default |
| -------------- | -------------------------- | ------- |
| `file`         | SRT file                   | -       |
| `language`     | Target language ISO code   | `zh`    |
| `prompt`       | Custom user prompt         | -       |
| `instructions` | Custom system instructions | -       |

## CLI Flags

Global flags (available on all subcommands):

| Flag        | Short | Description          | Default |
| ----------- | ----- | -------------------- | ------- |
| `--verbose` | `-v`  | Enable debug logging | `false` |

### subtrans

| Flag                  | Short | Description                                                   | Default              |
| --------------------- | ----- | ------------------------------------------------------------- | -------------------- |
| `--language`          | `-l`  | Target language ISO code                                      | `zh`                 |
| `--output`            | `-o`  | Output file path                                              | `<input>.<lang>.srt` |
| `--model`             | `-m`  | Model                                                         | -                    |
| `--max-batch-size`    |       | Lines per batch                                               | `30`                 |
| `--batch-split-punct` |       | Punctuation for splitting                                     | `.`                  |
| `--instructions`      |       | Path to instructions file                                     | -                    |
| `--prompt`            |       | Custom user prompt                                            | -                    |
| `--temperature`       |       | Temperature                                                   | `0.0`                |
| `--max-retries`       |       | API retry count                                               | `3`                  |
| `--include-original`  |       | Include original text in output                               | `false`              |
| `--strip-punctuation` |       | Strip trailing punctuation from translation and original text | `true`               |

### subtrans serve

| Flag     | Short | Description | Default     |
| -------- | ----- | ----------- | ----------- |
| `--host` |       | Listen host | `localhost` |
| `--port` | `-p`  | Listen port | `8091`      |

---

<div align="center">

Made with ❤️ by [heartleo](https://github.com/heartleo)

</div>
