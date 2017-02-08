package namespace

import (
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
		_, ok, err = n.IdExists(name)
		return err
	})
	return ok, err
}

// Override put on model
func (n *Namespace) Put() (err error) {
	return n.RunInTransaction(func() error {
		// Set key
		n.SetKey(n.Name)

		// Check if namespace exists
		ok, err := n.Exists()
		if err != nil && err != datastore.ErrNoSuchEntity {
			log.Warn("Failed to check for existence of namespace: %v", err)
			return err
		}

		// Warn if it already exists, otherwise save.
		if ok {
			log.Warn("Namespace exists: %v", n.Name)
			return NamespaceExists
		} else {
			return n.Model.Put()
		}
	}, datastore.TransactionOptions{XG: true})
}
