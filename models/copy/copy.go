package copy

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/models/ads"
)

func init() { orm.Register[Copy]("copy") }

type Type string

const (
	HeadlineType Type = "headline"
	ContentType  Type = "content"
)

type Copy struct {
	mixin.Model[Copy]
	AdIntegration

	Type Type   `json:"type" orm:"default:content"`
	Text string `json:"text" datastore:",noindex"`

	ParentCopyId string `json:"parentCopyId"`

	// Only for filters and searches, don't rely on this after you get it out
	// of the db. Always check m.ParentCopyId != "".
	IsParent bool `json:"isParent"`
}

func (m Copy) Fork() *Copy {
	m2 := New(m.Datastore())

	m2.AdIntegration = m.AdIntegration
	m2.Type = m.Type
	m2.Text = m.Text
	m2.ParentCopyId = m.Id()
	m2.IsParent = false

	return m2
}

func (m *Copy) Load(ps []datastore.Property) (err error) {
	// Ensure we're initialized
	if m.Type == "" {
		m.Type = ContentType
	}

	// Load supported properties
	return datastore.LoadStruct(m, ps)
}

func (m *Copy) Save() (ps []datastore.Property, err error) {
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

func New(db *datastore.Datastore) *Copy {
	a := new(Copy)
	a.Init(db)
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("copy")
}
