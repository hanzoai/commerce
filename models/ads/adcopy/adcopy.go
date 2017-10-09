package adcopy

import (
	"hanzo.io/models/mixin"
)

type AdCopy struct {
	mixin.Model

	Copy []byte `json:"copy"`
}
