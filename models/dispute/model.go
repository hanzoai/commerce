package dispute

import "github.com/hanzoai/commerce/datastore"

var kind = "dispute"

func (d Dispute) Kind() string {
	return kind
}

func (d *Dispute) Init(db *datastore.Datastore) {
	d.BaseModel.Init(db, d)
}

func (d *Dispute) Defaults() {
	d.Parent = d.Db.NewKey("synckey", "", 1, nil)
	if d.Status == "" {
		d.Status = NeedsResponse
	}
}

func New(db *datastore.Datastore) *Dispute {
	d := new(Dispute)
	d.Init(db)
	d.Defaults()
	return d
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
