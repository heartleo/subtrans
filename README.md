# subtrans

***使用 OpenAI API 翻译 SRT 字幕***

![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/heartleo/subtrans)](https://goreportcard.com/report/github.com/heartleo/subtrans)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

[中文](README.md) | [English](README_en.md)

## 特性

- 支持翻译 `.srt` 字幕文件至指定语言 [ISO 639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes)
- 支持 OpenAI 兼容 API
- 支持按句子边界智能分批
- 支持翻译缺失自动重试
- 支持自定义翻译指令和提示词
- 支持原文与译文对照输出（双语字幕）
- 支持作为 **命令行工具**、**Go 库** 使用
- 支持 HTTP API 及 SSE 流式输出

## 安装

```bash
go install github.com/heartleo/subtrans/cmd/subtrans-cli@latest
```

## 快速开始

### 命令行

```bash
# 创建env文件或设置环境变量
cat > .env <<EOF
OPENAI_API_KEY=sk-xxx
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4.1
EOF

# 默认翻译为中文
subtrans-cli input.srt

# 翻译为法语并指定输出文件
subtrans-cli -l fr -o output.fr.srt input.srt

# 使用自定义 API 地址和模型
export OPENAI_BASE_URL=https://your-api.com/v1
export OPENAI_MODEL=gpt-5.2
subtrans-cli -l fr input.srt
```

### Go 库

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

## 环境变量

| 变量                   | 说明     | 默认值                         |
|----------------------|--------|-----------------------------|
| `OPENAI_API_KEY`     | API 密钥 | -                           |
| `OPENAI_BASE_URL`    | API 地址 | `https://api.openai.com/v1` |
| `OPENAI_MODEL`       | 模型名称   | `gpt-4.1`                   |
| `OPENAI_TEMPERATURE` | 温度     | `0.0`                       |
| `OPENAI_MAX_RETRIES` | 最大重试次数 | `3`                         |

## Server API

```bash
# 启动服务
go run github.com/heartleo/subtrans/cmd/subtrans-server

# 自定义提示词
curl -X POST http://localhost:8091/translate \
  -F "file=@input.srt" \
  -F "language=fr" \
  -F "prompt=your-prompt"

# SSE 流式输出，逐批返回翻译结果
curl -X POST http://localhost:8091/translate \
  -H "Accept: text/event-stream" \
  -F "file=@input.srt" \
  -F "language=zh"
```

| 参数             | 说明          | 默认值  |
|----------------|-------------|------|
| `file`         | SRT 文件      | -    |
| `language`     | 目标语言 ISO 代码 | `zh` |
| `prompt`       | 自定义提示词      | -    |
| `instructions` | 自定义系统指令     | -    |

## 命令行参数

| 参数                    | 缩写   | 说明          | 默认值               |
|-----------------------|------|-------------|-------------------|
| `--language`          | `-l` | 目标语言 ISO 代码 | `zh`              |
| `--output`            | `-o` | 输出文件路径      | `<输入文件>.<语言>.srt` |
| `--model`             | `-m` | 模型          | -                 |
| `--max-batch-size`    |      | 每批行数        | `30`              |
| `--batch-split-punct` |      | 分批切分标点符号    | `.`               |
| `--instructions`      |      | 指令文本文件路径    | -                 |
| `--prompt`            |      | 自定义用户提示词    | -                 |
| `--temperature`       |      | 温度          | `0.0`             |
| `--max-retries`       |      | API失败重试次数   | `3`               |
| `--include-original`  |      | 输出中包含原文     | `false`           |
| `--strip-punctuation` |      | 去除尾部标点      | `true`            |
| `--verbose`           | `-v` | 启用调试日志      | `false`           |

---

<div align="center">

Made with ❤️ by [heartleo](https://github.com/heartleo)

</div>
