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
	*mixin.Model `datastore:"-"`

	OrganizationId  string
	Approved        bool
	Enabled         bool
	Category        Category
	Title           string
	Tagline         string
	PitchMedia      string
	VideoUrl        string
	VideoOverlayUrl string
	ImageUrl        string
	Description     string
	Backers         int
	Raised          int64
	Thumbnail       string
	OriginalUrl     string
	StoreUrl        string
	ProductIds      []string `datastore:"-"`
	MemberIds       []string `datastore:"-"`

	GoogleAnalytics string
	FacebookTag     string
	Links           []string
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Model = mixin.NewModel(db, c)
	return c
}

func (c Campaign) Kind() string {
	return "campaign2"
}
