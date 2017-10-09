package adset

import (
	"hanzo.io/models/mixin"
)

type FacebookAdSet struct {
}

type AdSet struct {
	mixin.Model
	FacebookAdSet
}
