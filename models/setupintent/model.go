package setupintent

import "github.com/hanzoai/commerce/datastore"

var kind = "setup-intent"

func (si SetupIntent) Kind() string {
	return kind
}

func (si *SetupIntent) Init(db *datastore.Datastore) {
	si.BaseModel.Init(db, si)
}

func (si *SetupIntent) Defaults() {
	si.Parent = si.Db.NewKey("synckey", "", 1, nil)
	if si.Status == "" {
		si.Status = RequiresPaymentMethod
	}
	if si.Usage == "" {
		si.Usage = "off_session"
	}
}

func New(db *datastore.Datastore) *SetupIntent {
	si := new(SetupIntent)
	si.Init(db)
	si.Defaults()
	return si
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
