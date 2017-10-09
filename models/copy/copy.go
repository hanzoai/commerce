package copy

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"

	. "hanzo.io/models/ads"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	HeadlineType Type = "headline"
	ContentType  Type = "content"
)

type Copy struct {
	mixin.Model
	AdIntegration

	Type Type   `json:"type"`
	Text string `json:"text" datastore:",noindex"`

	ParentCopyId string `json:"parentCopyId"`

	// Only for filters and searches, don't rely on this after you get it out
	// of the db. Always check m.ParentCopyId != "".
	IsParent bool `json:"isParent"`
}

func (m *Copy) Fork() *Copy {
	m2 := New(m.Db)
	m2.ParentCopyId = m.Id()
	return m2
}

func (m *Copy) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	m.Defaults()

	// Load supported properties
	return IgnoreFieldMismatch(aeds.LoadStruct(m, c))
}

func (m *Copy) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	m.IsParent = m.ParentCopyId != ""

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(m, c))
}

func (m Copy) GetParentCopyId() string {
	return m.ParentCopyId
}

func (m Copy) GetCopySearchFieldAndIds() (string, []string) {
	return "ParentCopyId", []string{m.Id()}
}
