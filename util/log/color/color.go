package color

import (
	"io"

	. "github.com/op/go-logging"
)

var (
	colors = []string{
		CRITICAL: ColorSeq(ColorMagenta),
		ERROR:    ColorSeq(ColorRed),
		WARNING:  ColorSeq(ColorYellow),
		NOTICE:   ColorSeq(ColorGreen),
		INFO:     ColorSeq(ColorWhite),
		DEBUG:    ColorSeq(ColorCyan),
	}
	boldcolors = []string{
		CRITICAL: ColorSeqBold(ColorMagenta),
		ERROR:    ColorSeqBold(ColorRed),
		WARNING:  ColorSeqBold(ColorYellow),
		NOTICE:   ColorSeqBold(ColorGreen),
		INFO:     ColorSeq(ColorWhite),
		DEBUG:    ColorSeqBold(ColorCyan),
	}
)

func FmtVerbLevel(layout string, level Level, output io.Writer) {
	if layout == "bold" {
		output.Write([]byte(boldcolors[level]))
	} else if layout == "reset" {
		output.Write([]byte("\033[0m"))
	} else {
		output.Write([]byte(colors[level]))
	}
}
