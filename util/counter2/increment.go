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
func Increment(c appengine.Context, name string) error {
	return IncrementBy(c, name, 1)
}

// Increment increments the named counter by amount
func IncrementBy(c appengine.Context, name string, amount int) error {
	IncrementByTask.Call(c, name, amount)
	return nil
}

// Increment hour suffixed key
func IncrementHour(ctx appengine.Context, key string, t time.Time) error {
	key += sep + hour(t)
	return Increment(ctx, key)
}

// Increment day suffixed key
func IncrementDay(ctx appengine.Context, key string, t time.Time) error {
	key += sep + day(t)
	return Increment(ctx, key)
}

// Increment month suffixed key
func IncrementMonth(ctx appengine.Context, key string, t time.Time) error {
	key += sep + month(t)
	return Increment(ctx, key)
}
