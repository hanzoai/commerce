package mixin

import (
	"time"

	counter "hanzo.io/util/counter2"
	"hanzo.io/util/log"
)

type Countable interface {
	Increment()
	IncrementDaily()
	IncrementHourly()
	IncrementMonthly()
}

type Counter struct {
	Entity Entity `json:"-" datastore:"-"`
}

func (c *Counter) Init(e Entity) {
	c.Entity = e
}

func (c *Counter) IncrementTotal(t time.Time) {
	if err := counter.Increment(c.Entity.Context(), c.Entity.Kind()+"-total-counter", c.Entity.Kind()); err != nil {
		log.Warn("IncrementTotal Counter Error %s", err, c.Entity.Context())
	}
}

func (c *Counter) IncrementHourly(t time.Time) {
	if err := counter.IncrementHourly(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"-hourly-counter", t); err != nil {
		log.Warn("IncrementHourly Counter Error %s", err, c.Entity.Context())
	}
}

// func (c *Counter) IncrementDaily() {
// 	counter.IncrementDaily(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"Counter", time.Now())
// }

func (c *Counter) IncrementMonthly(t time.Time) {
	if err := counter.IncrementMonthly(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"-monthly-counter", t); err != nil {
		log.Warn("IncrementMonthly Counter Error %s", err, c.Entity.Context())
	}
}

func (c *Counter) IncrementByTotal(amount int, t time.Time) {
	if err := counter.IncrementBy(c.Entity.Context(), c.Entity.Kind()+"-total-counter", c.Entity.Kind(), amount); err != nil {
		log.Warn("IncrementByTotal Counter Error %s", err, c.Entity.Context())
	}
}

func (c *Counter) IncrementByHourly(amount int, t time.Time) {
	if err := counter.IncrementByHourly(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"-hourly-counter", amount, t); err != nil {
		log.Warn("IncrementByHourly Counter Error %s", err, c.Entity.Context())
	}
}

// func (c *Counter) IncrementDaily() {
// 	counter.IncrementDaily(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"Counter", time.Now())
// }

func (c *Counter) IncrementByMonthly(amount int, t time.Time) {
	if err := counter.IncrementByMonthly(c.Entity.Context(), c.Entity.Kind(), c.Entity.Kind()+"-monthly-counter", amount, t); err != nil {
		log.Warn("IncrementByMonthly Counter Error %s", err, c.Entity.Context())
	}
}
