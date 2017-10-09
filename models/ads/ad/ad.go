package ad

import (
	"hanzo.io/models/mixin"
)

type FacebookAd struct {
}

type Ad struct {
	mixin.Model
	FacebookAd

	AdConfigId string `json:"adConfigId"`
}
