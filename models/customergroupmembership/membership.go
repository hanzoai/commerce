package customergroupmembership

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[CustomerGroupMembership]("customergroupmembership") }

type CustomerGroupMembership struct {
	mixin.EntityBridge[CustomerGroupMembership]

	CustomerGroupId string `json:"customerGroupId"`
	UserId          string `json:"userId"`
}

// New creates a new CustomerGroupMembership wired to the given datastore.
func New(db *datastore.Datastore) *CustomerGroupMembership {
	m := new(CustomerGroupMembership)
	m.Init(db)
	return m
}

// Query returns a datastore query for memberships.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("customergroupmembership")
}
