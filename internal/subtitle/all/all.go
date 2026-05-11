// Package all blank-imports every built-in subtitle codec so that
// subtitle.For / subtitle.DetectByExt can resolve any supported format.
// Import it from main/entrypoint packages with `_`.
package all

import (
	_ "github.com/heartleo/subtrans/internal/subtitle/ass"
	_ "github.com/heartleo/subtrans/internal/subtitle/lrc"
	_ "github.com/heartleo/subtrans/internal/subtitle/sbv"
	_ "github.com/heartleo/subtrans/internal/subtitle/srt"
	_ "github.com/heartleo/subtrans/internal/subtitle/vtt"
)
