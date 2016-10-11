package namespace

import (
	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/util/log"
)

type Namespace struct {
	mixin.Model

	IntId int64
	Name  string
}

func (n *Namespace) NameExists(name string) (ok bool, err error) {
	n.RunInTransaction(func() error {
		_, ok, err = n.Model.KeyExists(name)
		return err
	})

	return ok, err
}

// Override put on model
func (n *Namespace) Put() (err error) {
	return aeds.RunInTransaction(n.Db.Context, func(ctx appengine.Context) error {
		// Set key
		n.SetKey(n.Name)

		// Check if namespace exists
		ok, err := n.Exists()
		if err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}

		// Warn if it already exists, otherwise save.
		if ok {
			log.Warn("Namespace exists: %v", n.Name)
			return NamespaceExists
		} else {
			return n.Model.Put()
		}
	}, &aeds.TransactionOptions{XG: true})
}
