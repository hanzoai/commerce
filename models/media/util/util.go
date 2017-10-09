package util

import (
	"errors"

	"hanzo.io/datastore"
	"hanzo.io/models/media"
)

var NoParentMediaSet = errors.New("No Parent Media Found")

func GetParentMedia(db *datastore.Datastore, h HasParentMedia) (*media.Media, error) {
	if id := h.GetParentMediaId(); id == "" {
		return nil, NoParentMediaSet
	} else {
		m := media.New(db)
		err := m.GetById(id)
		return m, err
	}
}
