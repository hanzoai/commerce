package mixin

import (
	"time"

	counter "hanzo.io/util/counter2"
)

type Countable interface {
	Increment()
	IncrementDay()
	IncrementHour()
	IncrementMonth()
}

type Counter struct {
	Entity Entity `json:"-" datastore:"-"`
}

func (c *Counter) Init(e Entity) {
	c.Entity = e
}

func (c *Counter) Increment() {
	counter.Increment(c.Entity.Context(), c.Entity.Kind())
}

func (c *Counter) IncrementHour() {
	counter.IncrementHour(c.Entity.Context(), c.Entity.Kind(), time.Now())
}

func (c *Counter) IncrementDay() {
	counter.IncrementDay(c.Entity.Context(), c.Entity.Kind(), time.Now())
}

func (c *Counter) IncrementMonth() {
	counter.IncrementMonth(c.Entity.Context(), c.Entity.Kind(), time.Now())
}
