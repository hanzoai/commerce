package datastore

import aeds "google.golang.org/appengine/datastore"

// dst should be a pointer
func LoadStruct(dst interface{}, ps []aeds.Property) error {
	return IgnoreFieldMismatch(aeds.LoadStruct(dst, ps))
}

// src should be a pointer
func SaveStruct(src interface{}) ([]aeds.Property, error) {
	ps, err := aeds.SaveStruct(src)
	return ps, IgnoreFieldMismatch(err)
}
