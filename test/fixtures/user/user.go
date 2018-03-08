package user

import "hanzo.io/models/mixin"

type User struct {
	mixin.Model

	Name   string
	Hidden string `json:"-"`
	Count  int    `json:"-"`
	Count2 int    `json:"-"`

	C string `json:"-"`
	U string `json:"-"`
	D string `json:"-"`
}

func (u *User) Defaults() {
	u.Name = "Nobody"
}
