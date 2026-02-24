package campaign

import (
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/category"
	"github.com/hanzoai/orm"

	"github.com/hanzoai/commerce/datastore"
)

func init() { orm.Register[Campaign]("campaign") }

type Campaign struct {
	mixin.EntityBridge[Campaign]

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
	ProductIds      []string          `datastore:"-" json:"productIds" orm:"default:[]"`

	GoogleAnalytics string   `json:"googleAnalytics"`
	FacebookTag     string   `json:"facebookTag"`
	Links           []string `json:"links" orm:"default:[]"`
}

func New(db *datastore.Datastore) *Campaign {
	c := new(Campaign)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("campaign")
}
