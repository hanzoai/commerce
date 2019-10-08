package types

import "errors"

var (
	UserDoesNotExist = errors.New("User does not exist")
	UserNotProvided  = errors.New("None of User, User.id, Order.userId is set")
)
