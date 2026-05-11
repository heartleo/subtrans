package subtrans

import "github.com/heartleo/subtrans/internal/subtitle"

// Format identifies a supported subtitle format. Pass to [WithFormat].
type Format = subtitle.Format

// Supported subtitle formats.
const (
	FormatSRT = subtitle.FormatSRT
	FormatVTT = subtitle.FormatVTT
	FormatASS = subtitle.FormatASS
	FormatLRC = subtitle.FormatLRC
	FormatSBV = subtitle.FormatSBV
)

// DetectFormat maps a filename's extension to a [Format].
// Returns an error wrapping [ErrUnknownFormat] when the extension is unknown.
func DetectFormat(path string) (Format, error) {
	return subtitle.DetectByExt(path)
}

// ErrUnknownFormat is returned by [DetectFormat] when the extension cannot be
// resolved to a supported format.
var ErrUnknownFormat = subtitle.ErrUnknownFormat
