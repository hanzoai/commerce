package campaign

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/category"
	"crowdstart.com/util/val"
)

type Campaign struct {
	mixin.Model

	Slug string `json:"slug"`

	OrganizationId  string            `json:"organizationId"`
	Approved        bool              `json:"approved"`
	Enabled         bool              `json:"enabled"`
	Category        category.Category `json:"category"`
	Title           string            `json:"title"`
	Tagline         string            `json:"tagline"`
	PitchMedia      string            `json:"pitchMedia"`
	VideoUrl        string            `json:"videoUrl"`
	VideoOverlayUrl string            `json:"videoOverlayUrl"`
	ImageUrl        string            `json:"imageUrl"`
	Description     string            `json:"Description"`
	Backers         int               `json:"backers"`
	Raised          int64             `json:"raised"`
	Thumbnail       string            `json:"thumbnail"`
	OriginalUrl     string            `json:"originalUrl"`
	StoreUrl        string            `json:"storeUrl"`
	ProductIds      []string          `datastore:"-" json:"productIds"`

	GoogleAnalytics string   `json:"googleAnalytics"`
	FacebookTag     string   `json:"facebookTag"`
	Links           []string `json:"links"`
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Model = mixin.Model{Db: db, Entity: c}
	c.Links = make([]string, 0)
	c.ProductIds = make([]string, 0)
	return c
}

func (c Campaign) Kind() string {
	return "campaign"
}

func (c *Campaign) Validator() *val.Validator {
	return val.New()
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
