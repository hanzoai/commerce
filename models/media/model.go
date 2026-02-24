package media

import "github.com/hanzoai/commerce/datastore"

var kind = "media"

func (a Media) Kind() string {
	return kind
}

func (a *Media) Init(db *datastore.Datastore) {
	a.BaseModel.Init(db, a)
}

func (a *Media) Defaults() {
	a.Type = ImageType
	a.Usage = UnknownUsage
	a.IsParent = false
}

func New(db *datastore.Datastore) *Media {
	a := new(Media)
	a.Init(db)
	a.Defaults()
	return a
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
