// Copyright © 2026 Hanzo AI. MIT License.

package rate

import (
	"sync"

	"github.com/hanzoai/commerce/datastore"
)

// seedMu serializes reconciles. Two boots racing would each read "absent" and
// both insert, and a duplicate rate is a charge nobody can explain.
var seedMu sync.Mutex

// Seed reconciles the meter authority to the published rate file.
//
// Safe to run on EVERY boot: one point query per row, and a write only when the
// row is absent or has drifted from the file. It is how a rate reaches the
// database the first time, and how a correction in the file reaches a row nobody
// has touched.
//
// AN ADMIN EDIT OUTRANKS THE FILE, and that is the whole point of the authority.
// A row a person changed through admin.hanzo.ai is skipped — otherwise the next
// boot would silently revert a price someone set deliberately, which is the
// failure that makes an editable catalog worse than a hardcoded one: you would
// change a rate, watch it take effect, and find it gone after a restart with no
// record of what happened.
//
// Returns (created, corrected) so a caller can log what a boot actually did.
func Seed(db *datastore.Datastore, rows []*Rate) (created, corrected int, err error) {
	seedMu.Lock()
	defer seedMu.Unlock()

	for _, r := range rows {
		if r == nil || r.Product == "" || r.Meter == "" {
			continue // a rate with no identity cannot be reconciled against
		}

		r.Bind()
		existing := New(db)
		ok, qerr := existing.Query().Filter("Slug=", r.Slug).Get()
		if qerr != nil {
			return created, corrected, qerr
		}

		if ok {
			if existing.AdminEdited {
				continue // the operator's price wins
			}
			if equal(existing, r) {
				continue // already the published value — write nothing
			}
			copyInto(existing, r)
			if uerr := existing.Update(); uerr != nil {
				return created, corrected, uerr
			}
			corrected++
			continue
		}

		e := New(db)
		copyInto(e, r)
		if cerr := e.Create(); cerr != nil {
			return created, corrected, cerr
		}
		created++
	}
	return created, corrected, nil
}

// equal reports whether the stored row already says what the file says. Compared
// field by field rather than by a serialization, so a formatting change in the
// file does not rewrite every row and report hundreds of corrections.
func equal(a, b *Rate) bool {
	return a.Product == b.Product &&
		a.Meter == b.Meter &&
		a.Label == b.Label &&
		a.Unit == b.Unit &&
		a.Rate == b.Rate &&
		a.Currency == b.Currency &&
		a.Source == b.Source &&
		a.Included == b.Included &&
		// Status is compared only when the file states one, because copyInto
		// writes it only then. equal and copyInto have to agree about every
		// field or a reseed reports a correction, rewrites the row, and does it
		// again on the next boot — the row never converges and the audit trail
		// fills with writes that change nothing.
		(b.Status == "" || a.Status == b.Status)
}

// copyInto writes the published values onto a row, leaving its identity and its
// datastore key alone. AdminEdited is deliberately NOT copied: it is a fact
// about the ROW, not about the file, and copying it would let a seed clear the
// flag that protects an operator's edit.
func copyInto(dst, src *Rate) {
	// The parts first, then Bind, because the slug is DERIVED from them and a
	// bind over stale parts writes the previous row's identity.
	dst.Product = src.Product
	dst.Meter = src.Meter
	dst.Bind()
	dst.Label = src.Label
	dst.Unit = src.Unit
	dst.Rate = src.Rate
	dst.Currency = src.Currency
	dst.Source = src.Source
	dst.Included = src.Included
	if src.Status != "" {
		dst.Status = src.Status
	}
}
