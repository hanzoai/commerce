package util

import (
	"errors"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/media"
)

var NoParentMediaFound = errors.New("No Parent Media Found")

func GetParentMedia(db *datastore.Datastore, h BelongsToParentMedia) (*media.Media, error) {
	if id := h.GetParentMediaId(); id == "" {
		return nil, NoParentMediaFound
	} else {
		m := media.New(db)
		err := m.GetById(id)
		return m, err
	}
}

func GetMedias(db *datastore.Datastore, h HasMedias) ([]*media.Media, error) {
	field, keys := h.GetMediaSearchFieldAndIds()
	results := make([]*media.Media, 0)
	part := make([]*media.Media, 0)
	for _, key := range keys {
		if _, err := media.Query(db).Filter(field+"=", key).GetAll(&part); err != nil {
			return nil, err
		}
		results = append(results, part...)
	}
	return results, nil
}
