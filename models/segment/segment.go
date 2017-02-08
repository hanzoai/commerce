package segment

import (
	"hanzo.io/models/mixin"
)

type Segment struct {
	mixin.Model

	Name string `json:"name"`
}
