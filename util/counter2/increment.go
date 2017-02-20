package counter

import (
	"strconv"
	"time"

	"appengine"
)

var sep = "-"

// Time format helpers
func hour(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	return "hour" + sep + strconv.FormatInt(t2.Unix(), 10)
}

func day(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return "day" + sep + strconv.FormatInt(t2.Unix(), 10)
}

func month(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	return "month" + sep + strconv.FormatInt(t2.Unix(), 10)
}

// Increment increments the named counter by 1
func Increment(c appengine.Context, name, tag string) error {
	return IncrementBy(c, name, tag, 1)
}

// Increment hour suffixed name
func IncrementHourly(ctx appengine.Context, name, tag string, t time.Time) error {
	name += sep + hour(t)
	return Increment(ctx, name, tag)
}

// Increment day suffixed name
func IncrementDaily(ctx appengine.Context, name, tag string, t time.Time) error {
	name += sep + day(t)
	return Increment(ctx, name, tag)
}

// Increment month suffixed name
func IncrementMonthly(ctx appengine.Context, name, tag string, t time.Time) error {
	name += sep + month(t)
	return Increment(ctx, name, tag)
}

// Increment increments the named counter by amount
func IncrementBy(c appengine.Context, name, tag string, amount int) error {
	IncrementByTask.Call(c, name, tag, amount)
	return nil
}

// Increment hour suffixed name
func IncrementByHourly(ctx appengine.Context, name, tag string, amount int, t time.Time) error {
	name += sep + hour(t)
	return IncrementBy(ctx, name, tag, amount)
}

// Increment day suffixed name
func IncrementByDaily(ctx appengine.Context, name, tag string, amount int, t time.Time) error {
	name += sep + day(t)
	return IncrementBy(ctx, name, tag, amount)
}

// Increment month suffixed name
func IncrementByMonthly(ctx appengine.Context, name, tag string, amount int, t time.Time) error {
	name += sep + month(t)
	return IncrementBy(ctx, name, tag, amount)
}
