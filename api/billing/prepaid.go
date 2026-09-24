package billing

// prepaid.go — THE ONE WAY a subject's prepaid money is read and spent.
//
// Prepaid money is two things, drawn in one order: the subject's credit grants
// first, then its balance. The balance lives on the one ledger the host keeps —
// the ledger injected through creditledger when commerce is embedded (the same
// account an admin grant credits, the balance endpoint reads and the AI gate
// debits) — and on commerce's own "iam-user" transactions when it runs alone,
// which is where every deposit, top-up and credit it takes itself lands.
//
// A sale paid from the balance, a renewal the engine collects and the tier's
// balance all go through here, so none of them can read one ledger while another
// writes a second. The engine reaches it as an engine.Prepaid, bound to one org.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
)

// prepaid is one org's prepaid money as an engine.Prepaid.
type prepaid struct {
	org *organization.Organization
	db  *datastore.Datastore
}

var _ engine.Prepaid = prepaid{}

// prepaidFor binds the prepaid money of org's subjects to org's own store and
// books. The org is what names the ledger and whether its books are the sandbox.
func prepaidFor(ctx context.Context, org *organization.Organization) prepaid {
	return prepaid{org: org, db: datastore.New(org.Namespaced(ctx))}
}

// errShort is a draw the subject's prepaid money cannot cover. It moved nothing.
var errShort = creditledger.ErrShort

// Available is the subject's unspent credit grants plus its balance.
func (p prepaid) Available(ctx context.Context, subject string, cur currency.Type) (int64, error) {
	credits, err := creditsAvailable(p.db, subject)
	if err != nil {
		return 0, fmt.Errorf("read credit grants: %w", err)
	}
	bal, err := p.balance(ctx, subject, cur)
	if err != nil {
		return 0, fmt.Errorf("read balance: %w", err)
	}
	return credits + bal, nil
}

// Draw takes exactly amount — credit grants first, then the balance — or takes
// nothing.
//
// It is exactly once per ref. The draw is guarded on (subject, ref) and the
// guard keeps what was drawn, so a retry of an act whose answer was lost gets
// that answer back instead of paying again. A ref that already paid a different
// amount is refused: the same key is not the same act.
//
// Covered is decided before anything moves. The balance leg is a single debit
// that is itself all or nothing, so the only way it can fail after the credits
// are burned is a spend landing in between, and then the burned credits are put
// back.
//
// A debit can land and lose its answer. So the split is recorded under the ref
// before the ledger is asked for anything, and every later attempt under that ref
// posts the same split: the ledger answers a repeated (ref, amount) with the
// posting it already holds. The split is forgotten only when the ledger was never
// asked, or said the ref holds nothing.
func (p prepaid) Draw(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (engine.Drawn, error) {
	if amount <= 0 {
		return engine.Drawn{}, nil
	}
	if strings.TrimSpace(ref) == "" {
		return engine.Drawn{}, errors.New("prepaid draw: a ref is required")
	}
	rec, replay, err := idempotencykey.Begin(p.db, "billing-draw:"+subject, ref)
	switch {
	case err != nil:
		return engine.Drawn{}, fmt.Errorf("prepaid draw: cannot tell a retry from a first attempt: %w", err)
	case replay && rec.Status == idempotencykey.StatusCompleted:
		var d engine.Drawn
		if err := json.Unmarshal([]byte(rec.Response), &d); err != nil {
			return engine.Drawn{}, fmt.Errorf("prepaid draw %s: unreadable receipt: %w", ref, err)
		}
		if d.Credit+d.Balance != amount {
			return engine.Drawn{}, fmt.Errorf("prepaid draw %s already paid %d cents, not %d", ref, d.Credit+d.Balance, amount)
		}
		return d, nil
	case replay:
		return engine.Drawn{}, fmt.Errorf("prepaid draw %s is already in progress", ref)
	}

	d, err := p.draw(ctx, subject, cur, amount, ref)
	if err != nil {
		_ = rec.Delete() // what the ledger may hold is the split record's to remember
		return engine.Drawn{}, err
	}
	if body, merr := json.Marshal(d); merr == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
	return d, nil
}

// draw is Draw without the guard: the split, then the move.
func (p prepaid) draw(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (engine.Drawn, error) {
	split, recorded, err := idempotencykey.Begin(p.db, "billing-split:"+subject, ref)
	switch {
	case err != nil:
		return engine.Drawn{}, fmt.Errorf("read the split drawn under %s: %w", ref, err)
	case recorded && split.Status != idempotencykey.StatusCompleted:
		return engine.Drawn{}, fmt.Errorf("the split for %s is being recorded", ref)
	}
	var d engine.Drawn
	if recorded {
		if err := json.Unmarshal([]byte(split.Response), &d); err != nil || d.Credit+d.Balance != amount {
			return engine.Drawn{}, fmt.Errorf("the split recorded under %s is not a draw of %d cents", ref, amount)
		}
	} else {
		if d, err = p.plan(ctx, subject, cur, amount); err != nil {
			_ = split.Delete()
			return engine.Drawn{}, err
		}
		body, merr := json.Marshal(d)
		if merr == nil {
			merr = idempotencykey.Complete(split, string(body))
		}
		if merr != nil {
			_ = split.Delete()
			return engine.Drawn{}, fmt.Errorf("record the split before it moves money: %w", merr)
		}
	}

	burned, err := burnGrants(p.db, subject, d.Credit, "")
	if err == nil && burned.total() < d.Credit {
		err = fmt.Errorf("%w: credit grants moved while drawing", errShort)
	}
	if err != nil {
		burned.restore()
		if !recorded {
			_ = split.Delete() // this attempt's split, and the ledger was never asked
		}
		return engine.Drawn{}, err
	}
	if d.Balance == 0 {
		return d, nil
	}
	if d.Ref, err = p.debit(ctx, subject, cur, d.Balance, ref); err != nil {
		burned.restore()
		if errors.Is(err, errShort) {
			_ = split.Delete() // the ledger holds nothing under this ref
		}
		return engine.Drawn{}, err
	}
	return d, nil
}

// plan splits amount between the subject's credit grants and its balance, credits
// first, and refuses — moving nothing — when the two together fall short.
func (p prepaid) plan(ctx context.Context, subject string, cur currency.Type, amount int64) (engine.Drawn, error) {
	credits, err := creditsAvailable(p.db, subject)
	if err != nil {
		return engine.Drawn{}, fmt.Errorf("read credit grants: %w", err)
	}
	d := engine.Drawn{Credit: min(credits, amount)}
	d.Balance = amount - d.Credit
	if d.Balance > 0 {
		bal, err := p.balance(ctx, subject, cur)
		if err != nil {
			return engine.Drawn{}, fmt.Errorf("read balance: %w", err)
		}
		if bal < d.Balance {
			return engine.Drawn{}, fmt.Errorf("%w: holds %d cents, the draw is %d", errShort, credits+bal, amount)
		}
	}
	return d, nil
}

// ledgerOrg is the org's name as the ledger keys it.
func ledgerOrg(org *organization.Organization) string {
	return strings.ToLower(strings.TrimSpace(org.Name))
}

// balance reads the subject's balance from the one ledger.
func (p prepaid) balance(ctx context.Context, subject string, cur currency.Type) (int64, error) {
	if led := creditledger.Get(); led != nil {
		return led.Balance(ctx, ledgerOrg(p.org), subject, string(cur), p.org.TestMode())
	}
	return p.stored(ctx, subject, cur)
}

// stored is the balance on commerce's own ledger: the "iam-user" transactions
// every deposit, top-up and credit it takes itself writes, in the org's
// namespace and books, less what holds keep back.
func (p prepaid) stored(ctx context.Context, subject string, cur currency.Type) (int64, error) {
	datas, err := txutil.GetTransactionsByCurrency(p.org.Namespaced(ctx), subject, transaction.IAMUserKind, cur, p.org.TestMode())
	if err != nil {
		return 0, err
	}
	if d, ok := datas.Data[cur]; ok {
		return max(int64(d.Balance-d.Holds), 0), nil
	}
	return 0, nil
}

// debit draws amount from the subject's balance on the one ledger, all or
// nothing, and answers the ledger's id for the posting.
func (p prepaid) debit(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (string, error) {
	if led := creditledger.Get(); led != nil {
		id, _, err := led.Debit(ctx, creditledger.DebitInput{
			Org:         ledgerOrg(p.org),
			Subject:     subject,
			Currency:    string(cur),
			Reason:      "subscription " + ref,
			Ref:         ref,
			AmountCents: amount,
			Test:        p.org.TestMode(),
		})
		return id, err
	}
	// Commerce's own ledger: a withdrawal of the kind the balance, the gate and
	// every deposit count, in the same namespace and books.
	avail, err := p.stored(ctx, subject, cur)
	if err != nil {
		return "", err
	}
	if avail < amount {
		return "", fmt.Errorf("%w: holds %d cents, the debit is %d", errShort, avail, amount)
	}
	t := transaction.New(p.db)
	t.Type = transaction.Withdraw
	t.SourceId = subject
	t.SourceKind = transaction.IAMUserKind
	t.Currency = cur
	t.Amount = currency.Cents(amount)
	t.Notes = "subscription " + ref
	t.Tags = "subscription"
	t.Test = p.org.TestMode()
	if err := t.Create(); err != nil {
		return "", fmt.Errorf("record withdrawal: %w", err)
	}
	return t.Id(), nil
}

// Return puts amount back on the subject's balance on the one ledger, once per
// ref. The host ledger's credit is idempotent on its key; on commerce's own
// ledger the deposit is stored under a key derived from ref, so a second return
// under the same ref finds the first. It is prepaid money the subject can spend,
// the same kind a top-up is.
func (p prepaid) Return(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (string, error) {
	if amount <= 0 {
		return "", nil
	}
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("prepaid return: a ref is required")
	}
	if led := creditledger.Get(); led != nil {
		id, _, err := led.Credit(ctx, creditledger.CreditInput{
			Org:            ledgerOrg(p.org),
			Subject:        subject,
			Currency:       string(cur),
			Reason:         "returned " + ref,
			Tag:            returnTag,
			IdempotencyKey: ref,
			AmountCents:    amount,
			Test:           p.org.TestMode(),
		})
		return id, err
	}
	sum := sha256.Sum256([]byte("prepaid-return\x00" + subject + "\x00" + ref))
	key := p.db.NewKey("transaction", "rtrn_"+hex.EncodeToString(sum[:16]), 0, p.db.NewKey("synckey", "", 1, nil))
	t := transaction.New(p.db)
	switch err := t.Get(key); {
	case err == nil:
		return t.Id(), nil
	case !errors.Is(err, datastore.ErrNoSuchEntity):
		return "", fmt.Errorf("read the return under %s: %w", ref, err)
	}
	t = transaction.New(p.db)
	if err := t.SetKey(key); err != nil {
		return "", err
	}
	t.Type = transaction.Deposit
	t.DestinationId = subject
	t.DestinationKind = transaction.IAMUserKind
	t.Currency = cur
	t.Amount = currency.Cents(amount)
	t.Notes = "returned " + ref
	t.Tags = returnTag
	t.Test = p.org.TestMode()
	if err := t.Create(); err != nil {
		return "", fmt.Errorf("record the return under %s: %w", ref, err)
	}
	return t.Id(), nil
}

// returnTag tags money given back from an invoice that was voided or written
// off. It is prepaid money (bucket.DepositKind), like a top-up.
const returnTag = "invoice-return"

// creditsAvailable is what the subject's active credit grants still hold.
func creditsAvailable(db *datastore.Datastore, subject string) (int64, error) {
	grants, err := getActiveGrants(db, subject)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, g := range grants {
		n += g.RemainingCents
	}
	return n, nil
}

// openPaid opens a regular subscription whose first period prepaid money has
// already paid — drawn is what paid it — and raises that period's invoice as
// paid. The invoice is what makes the row payment-backed, so a plan the catalog
// does not publish confers its tier from the first moment, and it moves the row
// on to its next period, which the engine collects from the same money when it
// falls due.
//
// Only opening the row can fail it. The money has moved by then, so a failure to
// record the invoice after the row exists is logged for reconciliation and the
// row stands, the same answer a card sale gives.
func openPaid(db *datastore.Datastore, p *plan.Plan, req *createSubscriptionRequest, promoPercent int, promoName string, drawn engine.Drawn) (*subscription.Subscription, *billinginvoice.BillingInvoice, error) {
	p.TrialPeriodDays = 0
	sub, err := createSubscription(db, p, req)
	if err != nil {
		return nil, nil, err
	}
	sub.ProviderType = "credit"
	// Stamped before the invoice is built: draftPeriodInvoice prices the promo
	// off the row, so stamping it after would invoice the period at full price.
	sub.DiscountPercent, sub.DiscountName = promoPercent, promoName
	method, ref := "credit", drawn.Ref
	if drawn.Balance > 0 {
		method = "balance"
	}
	if ref == "" {
		ref = "credit_burn"
	}
	inv, err := engine.CreatePaidFirstInvoice(db, sub, method, ref)
	if err != nil {
		log.Error("RECONCILE: subscription %s (subject=%s) was paid from prepaid money (ref=%s) and its first invoice was not recorded: %v",
			sub.Id(), req.UserId, ref, err)
	}
	if err := sub.Update(); err != nil {
		log.Error("RECONCILE: subscription %s (subject=%s) was not saved after its first invoice: %v", sub.Id(), req.UserId, err)
	}
	return sub, inv, nil
}
