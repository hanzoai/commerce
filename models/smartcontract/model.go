package smartcontract

import (
	"hanzo.io/datastore"
)

var kind = "smartcontract"

func (sc SmartContract) Kind() string {
	return kind
}

func (c *SmartContract) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func New(db *datastore.Datastore) *SmartContract {
	c := new(SmartContract)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
