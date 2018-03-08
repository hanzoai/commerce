package counter

import (
	"time"

	"golang.org/x/net/context"
)

// Increment increments the named counter by amount
func IncrementBy(c context.Context, key string, amount int) error {
	IncrementByTask.Call(c, key, amount)
	return nil
}

// Increment hour suffixed key
func IncrementHourBy(ctx context.Context, prefix string, t time.Time, amount int) error {
	key := Key(prefix, hour(t))
	return IncrementBy(ctx, key, amount)
}

// Increment day suffixed key
func IncrementDayBy(ctx context.Context, prefix string, t time.Time, amount int) error {
	key := Key(prefix, day(t))
	return IncrementBy(ctx, key, amount)
}

// Increment month suffixed key
func IncrementMonthBy(ctx context.Context, prefix string, t time.Time, amount int) error {
	key := Key(prefix, month(t))
	return IncrementBy(ctx, key, amount)
}
