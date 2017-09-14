package store

import (
	"hanzo.io/models/shippingrates"
	"hanzo.io/models/taxrates"
)

// Hooks
func (s *Store) AfterCreate() error {
	trs := taxrates.New(s.Db)
	trs.StoreId = s.Id()
	trs.MustCreate()

	srs := shippingrates.New(s.Db)
	srs.StoreId = s.Id()
	srs.MustCreate()
	return nil
}
