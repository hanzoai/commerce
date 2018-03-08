package counter

import (
	"strconv"
	"strings"
	"time"
)

type Period string

var Sep = "/"

const (
	Yearly  Period = "yearly"
	Monthly        = "monthly"
	Weekly         = "weekly"
	Daily          = "daily"
)

func Key(parts ...string) string {
	return strings.Join(parts, Sep)
}

// Time format helpers
func hour(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	return Key("hour", strconv.FormatInt(t2.Unix(), 10))
}

func day(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return Key("day", strconv.FormatInt(t2.Unix(), 10))
}

func month(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	return Key("month", strconv.FormatInt(t2.Unix(), 10))
}
