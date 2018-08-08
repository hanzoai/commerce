package email

import (
	. "hanzo.io/types"
)

type Subscriber struct {
	Email    Email `json:"email"`
	Metadata Map   `json:"metadata"`
}
