# subtrans

***使用 OpenAI API 翻译字幕文件（SRT / VTT / ASS / LRC / SBV）***

![Go Version](https://img.shields.io/badge/go-1.25%2B-blue)
[![Go Report Card](https://goreportcard.com/badge/github.com/heartleo/subtrans)](https://goreportcard.com/report/github.com/heartleo/subtrans)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/subtrans/release.yml)](https://github.com/heartleo/subtrans/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/subtrans)](https://github.com/heartleo/subtrans/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/subtrans/total)](https://github.com/heartleo/subtrans/releases)
![License](https://img.shields.io/badge/license-MIT-green)

[中文](README.md) | [English](README_en.md)

## 特性

- 支持多种字幕格式：**SRT**、**WebVTT**、**ASS/SSA**、**LRC**、**SBV**
- 支持 OpenAI 兼容 API
- 支持按句子边界智能分批
- 支持翻译缺失/合并自动重试
- 支持自定义翻译指令和提示词
- 支持原文与译文双语输出
- 支持作为 **命令行工具**、**Go 库** 使用
- 支持 HTTP API 及 SSE 流式输出

## 安装

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

**Go install** (需要 Go 1.25+):

```bash
go install github.com/heartleo/subtrans/cmd/subtrans@latest
```

**从源码编译:**

```bash
git clone https://github.com/heartleo/subtrans
cd subtrans
go build -o subtrans ./cmd/subtrans
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

# 默认翻译为中文（按后缀自动识别格式）
subtrans input.srt
subtrans input.vtt
subtrans lyrics.lrc
subtrans anime.ass

# 翻译为法语并指定输出文件
subtrans -l fr -o output.fr.srt input.srt

# 使用自定义 API 地址和模型
export OPENAI_BASE_URL=https://your-api.com/v1
export OPENAI_MODEL=gpt-5.2
subtrans -l fr input.srt
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

非 SRT 格式通过 `WithFormat` 指定：

```go
t.Translate(ctx, content, "zh", subtrans.WithFormat(subtrans.FormatVTT))
t.Translate(ctx, content, "zh", subtrans.WithFormat(subtrans.FormatASS))

// 或按文件名自动识别
f, _ := subtrans.DetectFormat("input.lrc")
t.Translate(ctx, content, "zh", subtrans.WithFormat(f))
```

## 支持的字幕格式

| 格式 | 扩展名        | 支持 |
| ---- | ------------- | ---- |
| SRT  | `.srt`        | ✅    |
| VTT  | `.vtt`        | ✅    |
| ASS  | `.ass`/`.ssa` | ✅    |
| LRC  | `.lrc`        | ✅    |
| SBV  | `.sbv`        | ✅    |

## 环境变量

| 变量                 | 说明         | 默认值                      |
| -------------------- | ------------ | --------------------------- |
| `OPENAI_API_KEY`     | API 密钥     | -                           |
| `OPENAI_BASE_URL`    | API 地址     | `https://api.openai.com/v1` |
| `OPENAI_MODEL`       | 模型名称     | `gpt-5.5`                   |
| `OPENAI_TEMPERATURE` | 温度         | `0.0`                       |
| `OPENAI_MAX_RETRIES` | 最大重试次数 | `3`                         |

## Server API

```bash
# 启动服务
subtrans serve

# 自定义提示词
curl -X POST http://localhost:8091/translate \
  -F "file=@input.srt" \
  -F "language=fr" \
  -F "prompt=your-prompt"

# 上传 VTT/ASS/LRC/SBV 字幕文件
curl -X POST http://localhost:8091/translate \
  -F "file=@anime.ass" \
  -F "language=zh"

# 显式指定格式
curl -X POST http://localhost:8091/translate \
  -F "file=@noext_file" \
  -F "format=vtt" \
  -F "language=zh"

# SSE 流式输出，逐批返回翻译结果
curl -X POST http://localhost:8091/translate \
  -H "Accept: text/event-stream" \
  -F "file=@input.srt" \
  -F "language=zh"
```

| 参数           | 说明                                      | 默认值     |
| -------------- | ----------------------------------------- | ---------- |
| `file`         | 字幕文件                                  | -          |
| `format`       | 字幕格式（`srt`/`vtt`/`ass`/`lrc`/`sbv`） | 按后缀识别 |
| `language`     | 目标语言 ISO 代码                         | `zh`       |
| `prompt`       | 自定义提示词                              | -          |
| `instructions` | 自定义系统指令                            | -          |

## 命令行参数

全局参数（翻译和 serve 均支持）：

| 参数        | 缩写 | 说明         | 默认值  |
| ----------- | ---- | ------------ | ------- |
| `--verbose` | `-v` | 启用调试日志 | `false` |

### subtrans

| 参数                  | 缩写 | 说明                                              | 默认值                       |
| --------------------- | ---- | ------------------------------------------------- | ---------------------------- |
| `--language`          | `-l` | 目标语言 ISO 代码                                 | `zh`                         |
| `--output`            | `-o` | 输出文件路径                                      | `<输入文件>.<语言>.<原后缀>` |
| `--model`             | `-m` | 模型                                              | -                            |
| `--max-batch-size`    |      | 每批行数                                          | `30`                         |
| `--batch-split-punct` |      | 分批切分标点符号                                  | `.`                          |
| `--instructions`      |      | 指令文本文件路径                                  | -                            |
| `--prompt`            |      | 自定义用户提示词                                  | -                            |
| `--temperature`       |      | 温度                                              | `0.0`                        |
| `--max-retries`       |      | API 失败重试次数                                  | `3`                          |
| `--include-original`  |      | 输出中包含原文                                    | `false`                      |
| `--strip-punctuation` |      | 去除译文及原文（`--include-original` 时）尾部标点 | `true`                       |

### subtrans serve

| 参数     | 缩写 | 说明     | 默认值      |
| -------- | ---- | -------- | ----------- |
| `--host` |      | 监听地址 | `localhost` |
| `--port` | `-p` | 监听端口 | `8091`      |

---

<div align="center">

Made with ❤️ by [heartleo](https://github.com/heartleo)

</div>
