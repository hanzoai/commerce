package namespace

import (
	"golang.org/x/net/context"
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/util/log"
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
	return aeds.RunInTransaction(n.Db.Context, func(ctx context.Context) error {
		// Set key
		n.SetKey(n.Name)

		// Check if namespace exists
		ok, err := n.Exists()
		if err != nil && err != datastore.KeyNotFound {
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
