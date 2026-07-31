package organization

import (
	"errors"
)

var (
	UserNotTopLevel = errors.New("User is not in the top level namespace.")
)
