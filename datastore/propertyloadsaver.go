package datastore

import aeds "google.golang.org/appengine/datastore"

type Property aeds.Property
type PropertyList []aeds.Property

// dst should be a pointer
func LoadStruct(dst interface{}, ps PropertyList) error {
	return IgnoreFieldMismatch(aeds.LoadStruct(dst, ps))
}

// src should be a pointer
func SaveStruct(src interface{}) (PropertyList, error) {
	ps, err := aeds.SaveStruct(src)
	return ps, IgnoreFieldMismatch(err)
}
