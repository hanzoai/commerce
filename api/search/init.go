package search

import (
	"hanzo.io/models/order"
	"hanzo.io/models/user"
)

func init() {
	searches = make(map[string]*Search, 0)
	Register(user.User{}, user.Document{})
	Register(order.Order{}, order.Document{})
}
