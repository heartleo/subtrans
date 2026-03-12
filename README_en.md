# subtrans

***Translate SRT subtitles using OpenAI API***

![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/heartleo/subtrans)](https://goreportcard.com/report/github.com/heartleo/subtrans)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

[中文](README.md) | [English](README_en.md)

## Features

- Translate `.srt` subtitle files to any [ISO 639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes) language
- Works with any OpenAI-compatible API
- Smart batch splitting by sentence boundaries
- Automatic retry for missing translations
- Customizable translation instructions and prompts
- Bilingual output (original + translated text)
- Use as a **CLI tool** or **Go library**
- HTTP API with SSE streaming support

## Installation

```bash
go install github.com/heartleo/subtrans/cmd/subtrans-cli@latest
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
subtrans-cli input.srt

# Translate to French with custom output path
subtrans-cli -l fr -o output.fr.srt input.srt

# Use a custom API base URL and model
export OPENAI_BASE_URL=https://your-api.com/v1
export OPENAI_MODEL=gpt-5.2
subtrans-cli -l fr input.srt
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

| Variable               | Description    | Default                       |
|------------------------|----------------|-------------------------------|
| `OPENAI_API_KEY`       | API key        | -                             |
| `OPENAI_BASE_URL`      | API base URL   | `https://api.openai.com/v1`   |
| `OPENAI_MODEL`         | Model name     | `gpt-4.1`                     |
| `OPENAI_TEMPERATURE`   | Temperature    | `0.0`                         |
| `OPENAI_MAX_RETRIES`   | Max retries    | `3`                           |

## Server API

```bash
# Start server
go run github.com/heartleo/subtrans/cmd/subtrans-server

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

| Parameter      | Description              | Default |
|----------------|--------------------------|---------|
| `file`         | SRT file                 | -       |
| `language`     | Target language ISO code | `zh`    |
| `prompt`       | Custom user prompt       | -       |
| `instructions` | Custom system instructions | -     |

## CLI Flags

| Flag                    | Short  | Description              | Default              |
|-------------------------|--------|--------------------------|----------------------|
| `--language`            | `-l`   | Target language ISO code | `zh`                 |
| `--output`              | `-o`   | Output file path         | `<input>.<lang>.srt` |
| `--model`               | `-m`   | Model                    | -                    |
| `--max-batch-size`      |        | Lines per batch          | `30`                 |
| `--batch-split-punct`   |        | Punctuation for splitting| `.`                  |
| `--instructions`        |        | Path to instructions file| -                    |
| `--prompt`              |        | Custom user prompt       | -                    |
| `--temperature`         |        | Temperature              | `0.0`                |
| `--max-retries`         |        | API retry count          | `3`                  |
| `--include-original`    |        | Include original text    | `false`              |
| `--strip-punctuation`   |        | Strip trailing punctuation| `true`              |
| `--verbose`             | `-v`   | Enable debug logging     | `false`              |

---

<div align="center">

Made with ❤️ by [heartleo](https://github.com/heartleo)

</div>
