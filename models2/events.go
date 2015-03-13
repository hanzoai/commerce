package models

import "time"

type Event struct {
	Type      string
	Desc      string
	CreatedAt time.Time
}
