package adheadline

import (
	"hanzo.io/models/mixin"
)

type AdHeadline struct {
	mixin.Model

	Headline []byte `json:"headline"`
}
