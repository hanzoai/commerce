package copy

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"

	. "hanzo.io/models/ads"
)

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

func (m Copy) Fork() *Copy {
	m2 := New(m.Db)

	m2.AdIntegration = m.AdIntegration
	m2.Type = m.Type
	m2.Text = m.Text
	m2.ParentCopyId = m.Id()
	m2.IsParent = false

	return m2
}

func (m *Copy) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized
	m.Defaults()

	// Load supported properties
	return datastore.LoadStruct(m, ps)
}

func (m *Copy) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	m.IsParent = m.ParentCopyId != ""

	// Save properties
	return datastore.SaveStruct(m)
}

func (m Copy) GetParentCopyId() string {
	return m.ParentCopyId
}

func (m Copy) GetCopySearchFieldAndIds() (string, []string) {
	return "ParentCopyId", []string{m.Id()}
}
