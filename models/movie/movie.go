package movie

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/val"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Prune down since Product Listing has a lot of this info now
type Movie struct {
	mixin.BaseModel

	// Unique human readable id
	Slug string `json:"slug"`
	EIDR string `json:"eidr,omitempty"`
	IMDB string `json:"imdb,omitempty"`

	// Product Name
	Name string `json:"name"`

	// Product headline
	Headline string `json:"headline" datastore:",noindex"`

	// Product Excerpt
	Excerpt string `json:"excerpt" datastore:",noindex"`

	// Product Description
	Description string `json:"description", datastore:",noindex"`

	// Product Media
	Header      Media   `json:"header"`
	Image       Media   `json:"image"`
	Screenshots []Media `json:"screenshots"`
	Trailers    []Media `json:"trailers"`

	Cast []string `json:"cast"`
	Crew []string `json:"crew"`

	// Is the product available
	Available bool `json:"available"`

	// Is product hidden from users
	Hidden bool `json:"hidden"`
}

func (m *Movie) Validator() *val.Validator {
	return val.New().
		Check("Slug").Exists().
		Check("EIDR").Exists().
		Check("IMDB").Exists()
}

func (m *Movie) Load(ps []datastore.Property) (err error) {
	// Ensure we're initialized
	m.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(m, ps); err != nil {
		return err
	}

	// Deserialize from datastore

	return err
}

func (m *Movie) Save() ([]datastore.Property, error) {

	// Save properties
	return datastore.SaveStruct(m)
}

func (m Movie) DisplayName() string {
	return DisplayTitle(m.Name)
}

func (m Movie) DisplayImage() Media {
	for _, media := range m.Screenshots {
		if media.Type == MediaImage {
			return media
		}
	}
	return Media{}
}
