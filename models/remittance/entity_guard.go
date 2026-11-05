package remittance

import "github.com/hanzoai/commerce/models/mixin"

// A field named Key would shadow Model[T].Key() and silently break the entity
// interface; these assertions make that a compile error.
var _ mixin.Entity = (*Remittance)(nil)
