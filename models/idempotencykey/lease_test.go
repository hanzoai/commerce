package idempotencykey

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/util/test/ae"
)

// A lease is held by one holder at a time; one whose holder stopped renewing for
// StartedTTL is taken over; and only its holder releases it, so a holder that
// lost it to a takeover releases nothing of the new holder's.
func TestLease_OneHolderAndOnlyItReleases(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "lease")

	first, held, err := Hold(db, "sale:", "acme")
	if err != nil || !held {
		t.Fatalf("first hold: %v %v", held, err)
	}
	if _, held, err := Hold(db, "sale:", "acme"); err != nil || held {
		t.Fatalf("a held lease was taken again: %v %v", held, err)
	}

	old := nowFn
	nowFn = func() time.Time { return time.Now().Add(StartedTTL + time.Minute) }
	second, held, err := Hold(db, "sale:", "acme")
	nowFn = old
	if err != nil || !held {
		t.Fatalf("a lease left for StartedTTL was not taken over: %v %v", held, err)
	}

	first.Release()
	if _, held, err := Hold(db, "sale:", "acme"); err != nil || held {
		t.Fatalf("the first holder's release freed the second holder's lease: %v %v", held, err)
	}
	second.Release()
	third, held, err := Hold(db, "sale:", "acme")
	if err != nil || !held {
		t.Fatalf("a released lease could not be taken: %v %v", held, err)
	}
	third.Release()
	third.Release()
}
