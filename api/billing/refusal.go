// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package billing

// refusal.go — what a card money move does when the card is refused, in one place:
// which answers are refusals, the gateway key a retry reaches the gateway under, and
// how many refusals a wallet may collect before its cards are no longer tried.

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/payment/processor"
)

// refusalOf is the processor's refusal of a charge, or nil when the processor did
// not refuse the card but failed to answer at all.
//
// A refusal is a processor.Decline, found in the error or in the result that carried
// it. A result that did not settle and states no Decline is a refusal with no code,
// which reads to the buyer as a plain decline. Nothing else is read: a processor's
// message is never searched for a reason, because the text beside a refusal can say
// anything (Square's carries the card's cvv_status on every decline), and an error
// that is not a refusal is the processor failing, which is ours to report and not the
// card's.
func refusalOf(res *processor.PaymentResult, err error) *processor.Decline {
	if d, ok := declineOf(res, err); ok {
		return d
	}
	if err == nil && res != nil && !res.Success {
		return &processor.Decline{}
	}
	return nil
}

// declinedCard is a refusal as an error: the buyer's sentence, with the processor's
// Decline beneath it for a caller that answers with the processor's code.
type declinedCard struct{ decline *processor.Decline }

func (e declinedCard) Error() string { return e.decline.Sentence() }
func (e declinedCard) Unwrap() error { return e.decline }

// errProcessorFailed marks a card money move the processor failed to answer: no
// refusal, and an outcome it did not state. It is the processor's fault and never the
// card's, so it is reported as an outage rather than as a decline, and it is
// processor.ErrUnknownOutcome, so a later attempt keeps the gateway key.
var errProcessorFailed = fmt.Errorf("the payment processor could not take the card: %w", processor.ErrUnknownOutcome)

// IsProcessorFailed reports whether err is a card money move the processor failed
// to answer, as distinct from a card it refused.
func IsProcessorFailed(err error) bool { return errors.Is(err, errProcessorFailed) }

// processorSentence is what a buyer reads when the processor failed to answer.
const processorSentence = "The payment processor could not take the card. Try again in a moment."

// processorFailure is a card money move the processor failed to answer. It is logged
// here with the processor's own error, which [thirdparty/square] has already reduced
// to a status and a code, and answers only [processorSentence].
func processorFailure(what, subject string, err error) error {
	log.Error("%s: the payment processor failed (subject=%s): %v", what, subject, err)
	return errProcessorFailed
}

// failedAttempt is [processorFailure] for a buyer's attempt, which is also counted
// against the wallet's window ([tally]).
func failedAttempt(db *datastore.Datastore, what, subject string, err error) error {
	tally(db, subject)
	return processorFailure(what, subject, err)
}

// ── the gateway key a retry reaches the gateway under ─────────────────────────

// A gateway stores its answer under the idempotency key it was sent, a refusal
// included, and answers the same key the same way for as long as it keeps it. The
// cores derive that key from facts that are stable across a retry, never from the
// card, so that a lost response retried with a re-tokenized card is not charged
// twice. After a REFUSAL that stability is wrong: nothing was charged, and the
// buyer's next attempt, with a corrected card or another one, must reach the gateway
// under a key of its own or the gateway answers it with the first refusal.
//
// So each refusal is counted under the key it was answered for, and the count is
// part of the next attempt's key. A retry after a failure that was not a refusal
// keeps its key, which is the de-duplication the key exists for.

// declinesScope names the count of refusals under one gateway key.
const declinesScope = "declined:"

// attemptKey is the gateway key for the next attempt under base: base itself before
// any refusal, and base with the refusal count after.
func attemptKey(db *datastore.Datastore, base string) string {
	if n := count(db, declinesScope, base); n > 0 {
		return base + ":r" + strconv.Itoa(n)
	}
	return base
}

// ── the failed attempts a wallet may collect ──────────────────────────────────

// A buyer's card attempts are held per wallet: one attempt at a time, and a wallet
// that has collected [declineCeiling] failed attempts in [declineWindow] has no more
// cards tried until the window turns, refused before the processor is asked. Testing
// stolen cards is a run of refusals against one account, so this bounds what one
// account can learn and what it costs the merchant at the processor, and a buyer
// whose own card is refused a few times is not affected. It is keyed on the wallet
// the charge would credit, which is the org for a tenant and the person in the shared
// signup org, so one stranger's refusals never lock out the rest of that org.
//
// A failed attempt is one the processor refused or failed to answer ([failedAttempt]):
// every outcome but a settled payment and one still processing. An attempt the
// processor turns away without a refusal (a spent or malformed token) holds the
// reservation as long as a refused one, so counting only refusals would let one
// member hold the wallet from every other member indefinitely. The cost is that an
// outage at the processor spends a wallet's ceiling as its buyers retry.
//
// ONE ATTEMPT AT A TIME is what makes the ceiling a ceiling. The count is read before
// a charge and written after it fails, so attempts that overlap all read the same
// count and all reach the processor. commerce is the single writer for each tenant,
// so this process holds every attempt a wallet makes, and a reservation in memory is
// the whole of what serialising them takes.
//
// ONLY A BUYER'S ATTEMPT IS HELD. The off-session charges (auto-recharge and
// renewals) charge a card the org already chose, are not a buyer testing anything,
// and must neither spend a buyer's ceiling nor be stopped by one.
//
// It is not per card: a card is known to this process only after the processor has
// been asked (a single-use token names no card), and counting it afterwards would
// mean keeping card identifiers the processor's answer is otherwise scrubbed of.

// declineCeiling is how many failed attempts one wallet may collect in [declineWindow].
const declineCeiling = 5

// declineWindow is the span declineCeiling counts over.
const declineWindow = time.Hour

// walletScope names a wallet's failed-attempt count for one window.
const walletScope = "declined-wallet:"

// errDeclineCeiling marks a charge refused before the processor was asked, because
// the wallet has collected declineCeiling failed attempts in this window.
var errDeclineCeiling = errors.New("too many declined card attempts; try again later")

// IsDeclineCeiling reports whether err is that refusal.
func IsDeclineCeiling(err error) bool { return errors.Is(err, errDeclineCeiling) }

// windowOf is the current window's key.
func windowOf() string {
	return strconv.FormatInt(time.Now().Unix()/int64(declineWindow/time.Second), 10)
}

// errAttemptInFlight marks a buyer's card attempt refused because another attempt
// for the same wallet is in progress.
var errAttemptInFlight = errors.New("another card attempt for this account is in progress")

// IsAttemptInFlight reports whether err is that refusal.
func IsAttemptInFlight(err error) bool { return errors.Is(err, errAttemptInFlight) }

// inFlight is the wallets with a buyer's card attempt in progress in this process.
var inFlight = struct {
	sync.Mutex
	wallets map[string]bool
}{wallets: map[string]bool{}}

// attempt reserves wallet subject of org for one buyer's card attempt, refusing when
// another is in progress or the wallet is at its ceiling. The caller releases the
// reservation once the attempt's refusal, if any, is recorded.
func attempt(db *datastore.Datastore, org, subject string) (release func(), err error) {
	key := org + "\x00" + subject
	inFlight.Lock()
	if inFlight.wallets[key] {
		inFlight.Unlock()
		return nil, errAttemptInFlight
	}
	inFlight.wallets[key] = true
	inFlight.Unlock()
	release = func() {
		inFlight.Lock()
		delete(inFlight.wallets, key)
		inFlight.Unlock()
	}
	if count(db, walletScope+subject, windowOf()) >= declineCeiling {
		release()
		return nil, errDeclineCeiling
	}
	return release, nil
}

// answered records the processor's answer to a charge that did not settle, made
// under gateway key base, and reports whether it was a refusal. A refusal moves the
// next attempt onto a key of its own ([rotate]); a payment still processing keeps
// its key, so a retry is answered with that payment rather than taking another. It
// logs the processor's category and code and nothing else of the answer. An empty
// base is an answer given with no gateway key (a card vaulted).
func answered(db *datastore.Datastore, what, subject, base string, d *processor.Decline) bool {
	if d.Processing() {
		log.Warn("%s: payment still processing (subject=%s): %s %s", what, subject, d.Category, d.Code)
		return false
	}
	log.Warn("%s: card declined (subject=%s): %s %s", what, subject, d.Category, d.Code)
	rotate(db, base)
	return true
}

// refused is [answered] for a buyer's attempt, and a refusal is also counted against
// the wallet's window ([tally]).
func refused(db *datastore.Datastore, what, subject, base string, d *processor.Decline) {
	if answered(db, what, subject, base, d) {
		tally(db, subject)
	}
}

// declineStatus is the HTTP status a buyer is answered for d: 402 for a refusal, and
// 409 for a payment still processing, which is neither the buyer's fault nor
// something another attempt should repeat.
func declineStatus(d *processor.Decline) int {
	if d.Processing() {
		return 409
	}
	return 402
}

// The sentences a buyer reads when an attempt is held.
const (
	ceilingSentence = "Too many declined card attempts. Try again later."
	attemptSentence = "Another card payment for this account is in progress. Try again in a moment."
)

// rotate moves the next attempt under gateway key base onto a key of its own
// ([attemptKey]), after a refusal answered under it.
func rotate(db *datastore.Datastore, base string) {
	if base != "" {
		add(db, declinesScope, base)
	}
}

// tally counts a failed attempt against wallet subject's window.
func tally(db *datastore.Datastore, subject string) {
	add(db, walletScope+subject, windowOf())
}

// count reads a count, zero when none is recorded or the store cannot say.
// The store failing open here is deliberate: a count that cannot be read costs a
// retry its fresh key or lifts the ceiling for one attempt, and refusing every
// charge whenever the store blinks would cost every buyer their purchase.
func count(db *datastore.Datastore, scope, key string) int {
	rec := idempotencykey.New(db)
	if rec.Get(db.NewKey(rec.Kind(), idempotencykey.DeterministicID(scope, key), 0, nil)) != nil {
		return 0
	}
	n, _ := strconv.Atoi(rec.Response)
	return n
}

// counts serialises every count's read-and-write in this process, the single writer
// for each tenant, so two attempts recorded at once are two.
var counts sync.Mutex

// add increments a count.
func add(db *datastore.Datastore, scope, key string) {
	counts.Lock()
	defer counts.Unlock()
	rec := idempotencykey.New(db)
	rec.SetId(idempotencykey.DeterministicID(scope, key))
	rec.Scope = scope
	rec.IdemKey = key
	rec.Status = "declined"
	rec.Response = strconv.Itoa(count(db, scope, key) + 1)
	if err := rec.Put(); err != nil {
		log.Error("recording a failed card attempt (%s%s): %v", scope, key, err)
	}
}
