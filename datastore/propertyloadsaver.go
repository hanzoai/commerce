package datastore

import aeds "google.golang.org/appengine/datastore"

// dst should be a pointer
func LoadStruct(dst interface{}, properties []aeds.Property) error {
	return IgnoreFieldMismatch(aeds.LoadStruct(dst, properties))
}

// src should be a pointer
func SaveStruct(src interface{}) ([]aeds.Property, error) {
	properties, err := aeds.SaveStruct(src)
	return properties, IgnoreFieldMismatch(err)
}
