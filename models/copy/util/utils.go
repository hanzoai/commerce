package util

import (
	"errors"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/copy"
)

var NoParentCopyFound = errors.New("No Parent Copy Found")

func GetParentCopy(db *datastore.Datastore, h BelongsToParentCopy) (*copy.Copy, error) {
	if id := h.GetParentCopyId(); id == "" {
		return nil, NoParentCopyFound
	} else {
		m := copy.New(db)
		err := m.GetById(id)
		return m, err
	}
}

func GetCopies(db *datastore.Datastore, h HasCopies) ([]*copy.Copy, error) {
	field, keys := h.GetCopySearchFieldAndIds()
	results := make([]*copy.Copy, 0)
	part := make([]*copy.Copy, 0)
	for _, key := range keys {
		if _, err := copy.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}
