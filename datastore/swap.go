package datastore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/mintauth"
)

// Swap writes val under keyOrKind only while the stored entity is still the one
// read at prev, its updatedAt (zero for an entity never stored): db.Swapper. It
// reports whether it wrote; false means another writer changed the entity since
// the read, and the caller reads again. A store that cannot swap refuses rather
// than fall back to a blind put.
func (d *Datastore) Swap(keyOrKind interface{}, prev time.Time, val interface{}) (bool, error) {
	if d.database == nil {
		return false, errors.New("datastore: database not initialized")
	}
	if err := mintauth.Enforce(d.Context, val); err != nil {
		d.warn("Refused unauthorized mint (%v, %#v): %v", convertKeyOrKind(d, keyOrKind), val, err, d.Context)
		return false, err
	}
	s, ok := d.database.(db.Swapper)
	if !ok {
		return false, fmt.Errorf("datastore: %T cannot swap", d.database)
	}
	stamp := ""
	if !prev.IsZero() {
		b, err := json.Marshal(prev)
		if err != nil {
			return false, err
		}
		stamp = strings.Trim(string(b), `"`)
	}
	dskey := convertKeyOrKind(d, keyOrKind)
	return s.Swap(d.Context, dskey.ToDBKey(d.database), stamp, val)
}
