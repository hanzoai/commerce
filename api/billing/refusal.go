// Copyright 2014-present Hanzo AI Inc. Licensed under MIT OR Apache-2.0.

package billing

// refusal.go — what a card money move does when the card is refused, in one place:
// which answers are refusals, the gateway key a retry reaches the gateway under, and
// how many refusals a wallet may collect before its cards are no longer tried.

import (
	"errors"
	"strconv"
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
// refusal, and no charge. It is the processor's fault and never the card's, so it is
// reported as an outage rather than as a decline.
var errProcessorFailed = errors.New("the payment processor could not take the card")

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

// ── the refusals a wallet may collect ─────────────────────────────────────────

// Refusals are also counted per wallet, per hour, and a wallet that has collected
// [declineCeiling] of them has no more cards tried until the hour turns: the charge
// is refused before the processor is asked. Testing stolen cards is a run of
// refusals against one account, so this bounds what one account can learn and what
// it costs the merchant at the processor, and a buyer whose own card is refused a few
// times is not affected. It is keyed on the wallet the charge would credit, which is
// the org for a tenant and the person in the shared signup org, so one stranger's
// refusals never lock out the rest of that org.
//
// It is not per card: a card is known to this process only after the processor has
// been asked (a single-use token names no card), and counting it afterwards would
// mean keeping card identifiers the processor's answer is otherwise scrubbed of.

// declineCeiling is how many refusals one wallet may collect in [declineWindow].
const declineCeiling = 5

// declineWindow is the span declineCeiling counts over.
const declineWindow = time.Hour

// walletScope names a wallet's refusal count for one window.
const walletScope = "declined-wallet:"

// errDeclineCeiling marks a charge refused before the processor was asked, because
// the wallet has collected declineCeiling refusals in this window.
var errDeclineCeiling = errors.New("too many declined card attempts; try again later")

// IsDeclineCeiling reports whether err is that refusal.
func IsDeclineCeiling(err error) bool { return errors.Is(err, errDeclineCeiling) }

// windowOf is the current window's key.
func windowOf() string {
	return strconv.FormatInt(time.Now().Unix()/int64(declineWindow/time.Second), 10)
}

// ceiling refuses a card money move for a wallet that has collected declineCeiling
// refusals in this window.
func ceiling(db *datastore.Datastore, subject string) error {
	if count(db, walletScope+subject, windowOf()) >= declineCeiling {
		return errDeclineCeiling
	}
	return nil
}

// refused records a refusal the processor answered under gateway key base for the
// wallet subject: once against the key, so the next attempt reaches the processor
// under a key of its own, and once against the wallet's window. An empty base is a
// refusal answered with no gateway key (a card vaulted), counted against the wallet
// alone. It logs the processor's category and code and nothing else of the answer.
func refused(db *datastore.Datastore, what, subject, base string, d *processor.Decline) {
	log.Warn("%s: card declined (subject=%s): %s %s", what, subject, d.Category, d.Code)
	if base != "" {
		add(db, declinesScope, base)
	}
	add(db, walletScope+subject, windowOf())
}

// count reads a refusal count, zero when none is recorded or the store cannot say.
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

// add increments a refusal count.
func add(db *datastore.Datastore, scope, key string) {
	rec := idempotencykey.New(db)
	rec.SetId(idempotencykey.DeterministicID(scope, key))
	rec.Scope = scope
	rec.IdemKey = key
	rec.Status = "declined"
	rec.Response = strconv.Itoa(count(db, scope, key) + 1)
	if err := rec.Put(); err != nil {
		log.Error("recording a card refusal (%s%s): %v", scope, key, err)
	}
}
