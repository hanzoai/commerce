package campaign

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
)

type Category string

const (
	Arts       Category = "arts"
	Fashion             = "fashion"
	Film                = "film"
	Food                = "food"
	Gaming              = "gaming"
	Health              = "health"
	Music               = "music"
	Sports              = "sports"
	Technology          = "technology"
)

type Campaign struct {
	mixin.Model

	OrganizationId  string   `json:"organizationId"`
	Approved        bool     `json:"approved"`
	Enabled         bool     `json:"enabled"`
	Category        Category `json:"category"`
	Title           string   `json:"title"`
	Tagline         string   `json:"tagline"`
	PitchMedia      string   `json:"pitchMedia"`
	VideoUrl        string   `json:"videoUrl"`
	VideoOverlayUrl string   `json:"videoOverlayUrl"`
	ImageUrl        string   `json:"imageUrl"`
	Description     string   `json:"Description"`
	Backers         int      `json:"backers"`
	Raised          int64    `json:"raised"`
	Thumbnail       string   `json:"thumbnail"`
	OriginalUrl     string   `json:"originalUrl"`
	StoreUrl        string   `json:"storeUrl"`
	ProductIds      []string `datastore:"-" json:"productIds"`

	GoogleAnalytics string   `json:"googleAnalytics"`
	FacebookTag     string   `json:"facebookTag"`
	Links           []string `json:"links"`
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Model = mixin.Model{Db: db, Entity: c}
	return c
}

func (c Campaign) Kind() string {
	return "campaign2"
}
