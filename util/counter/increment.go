package counter

import (
	"time"

	"golang.org/x/net/context"
)

// Increment increments the named counter by 1
func Increment(c context.Context, key string) error {
	return IncrementBy(c, key, 1)
}

// Increment hour suffixed key
func IncrementHour(ctx context.Context, prefix string, t time.Time) error {
	key := Key(prefix, hour(t))
	return Increment(ctx, key)
}

// Increment day suffixed key
func IncrementDay(ctx context.Context, prefix string, t time.Time) error {
	key := Key(prefix, day(t))
	return Increment(ctx, key)
}

// Increment month suffixed key
func IncrementMonth(ctx context.Context, prefix string, t time.Time) error {
	key := Key(prefix, month(t))
	return Increment(ctx, key)
}
