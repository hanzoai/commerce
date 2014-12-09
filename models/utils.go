package models

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	humanize "github.com/dustin/go-humanize"
)

func FloatPrice(price int64) float64 {
	return math.Floor(float64(price)*100+0.5) / 1000000
}

func DisplayPrice(price int64) string {
	f := strconv.FormatFloat(FloatPrice(price), 'f', 2, 64)
	bits := strings.Split(f, ".")
	decimal := bits[1]
	integer, _ := strconv.ParseInt(bits[0], 10, 64)
	return humanize.Comma(integer) + "." + decimal
}

// Non-breaking hyphens in title
func DisplayTitle(title string) string {
	return strings.Replace(title, "-", "&#8209;", -1)
}

func SplitParagraph(text string) []string {
	return regexp.MustCompile("\\n\\s*\\n").Split(text, -1)
}
