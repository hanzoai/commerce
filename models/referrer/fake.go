package referrer

import (
	"math/rand"
	"time"

	"github.com/hanzoai/commerce/datastore"
)

func Fake(db *datastore.Datastore, userId string) *Referrer {
	ref := New(db)
	// A minted code, not a dictionary word. A referral code is unique by
	// construction — the create handler reissues any code already present in
	// the namespace — and drawing one from fake's 249-word list meant two
	// referrers in a suite collided about one run in twenty. The server then
	// did its job, issued a different code, and the spec asserting "the server
	// kept the code I sent" failed for a reason that had nothing to do with the
	// server. Nor was it reproducible from the suite's seed: the words came
	// from the auto-seeded global math/rand.
	ref.Code = NewCode()
	ref.UserId = userId
	ref.FirstReferredAt = time.Date(rand.Intn(15)+2000, time.Month(rand.Intn(12)+1), rand.Intn(25)+1, 0, 0, 0, 0, time.UTC)
	return ref
}
