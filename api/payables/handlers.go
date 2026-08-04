// Package payables is the manual payables surface: what we owe, and how a human
// recorded paying it.
//
// Commerce accrues a payable (models/fee.Fee) whenever someone earns — an
// affiliate commission on an order, a referral revenue share on metered spend,
// an OSS contributor allocation. One payable shape for all of them: the program
// is fee.Type, a FIELD, and the payee is fee.PayeeId.
//
// Nothing here executes a payout. The owner pays out-of-band and records the
// fact; the record is a models/transfer.Transfer naming the payable it settles.
// A payment is appended, never a mutation of the payable — so what is still owed
// is a fold over the payments, partial payment needs no special case, and no
// replay can settle twice.
//
// Mounted under /v1 (api/api.go), platform-admin gated:
//
//	GET  /v1/payables                  -> what we owe, per payable and per payee
//	POST /v1/payables/:feeid/payments  -> record a payment against one payable
//
// The audit trail is the existing transfer REST surface (GET /v1/transfer),
// where every recorded payment lands.
//
// The sibling of api/costs: costs answers what we owe our VENDORS, this answers
// what we owe our PAYEES.
package payables

import (
	"strings"
	"time"

	"github.com/hanzoai/money"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/fee"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

// Route registers the payables endpoints. What we owe across every payee is a
// PLATFORM god-view, so on top of the route-level token gate each handler calls
// middleware.RequirePlatformAdmin — that route gate is a no-op on the IAM path
// and must never be trusted alone.
func Route(r zip.Router, args ...zip.Handler) {
	api := r.Group("payables")
	api.Use(middleware.TokenRequired(permission.Admin))

	api.Get("", List)
	api.Post("/:feeid/payments", RecordPayment)
}

// Payable is one thing we owe.
type Payable struct {
	Id          string         `json:"id"`
	Name        string         `json:"name"`
	Program     fee.Type       `json:"program"`
	PayeeId     string         `json:"payeeId"`
	Owed        currency.Money `json:"owed"`
	Paid        currency.Money `json:"paid"`
	Outstanding currency.Money `json:"outstanding"`
	Status      fee.Status     `json:"status"`
	AccruedAt   time.Time      `json:"accruedAt"`
	EarnedBy    string         `json:"earnedBy,omitempty"` // the charge or usage event
}

// PayeeTotal is what one payee is owed in one asset.
type PayeeTotal struct {
	PayeeId     string         `json:"payeeId"`
	Program     fee.Type       `json:"program"`
	Outstanding currency.Money `json:"outstanding"`
	Count       int            `json:"count"`
}

// ListResponse answers "what do we owe" at a glance.
type ListResponse struct {
	Payables []Payable        `json:"payables"`
	ByPayee  []PayeeTotal     `json:"byPayee"`
	Totals   []currency.Money `json:"totals"` // grand total, per asset
	Matured  int              `json:"matured"`
}

// paidByFee folds every recorded payment onto the payable it settles. What is
// owed is a question about the payments, so it is answered by reading them.
func paidByFee(db *datastore.Datastore) (map[string]money.Amount, error) {
	trs := make([]*transfer.Transfer, 0)
	if _, err := transfer.Query(db).GetAll(&trs); err != nil {
		return nil, err
	}
	paid := map[string]money.Amount{}
	for _, tr := range trs {
		if tr.FeeId == "" || tr.Settles.IsZero() {
			continue
		}
		amt, err := tr.Settles.Exact()
		if err != nil {
			continue
		}
		if prior, ok := paid[tr.FeeId]; ok {
			if sum, err := prior.Add(amt); err == nil {
				amt = sum
			}
		}
		paid[tr.FeeId] = amt
	}
	return paid, nil
}

// group accumulates one payee's outstanding total in one asset.
type group struct {
	payee   string
	program fee.Type
	total   money.Amount
	count   int
}

// add sums b into a, treating a zero-value (uninitialised) a as zero.
func add(a, b money.Amount) money.Amount {
	sum, err := a.Add(b)
	if err != nil {
		return b
	}
	return sum
}

// List answers what we owe.
//
//	GET /v1/payables?payee=&status=&program=
//
// It matures pending payables past the clawback buffer first (fee.Qualify), then
// reports. What we owe is a question about NOW, so it is computed when asked
// rather than by a cron nobody scheduled.
func List(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	matured, err := fee.Qualify(db, fee.DefaultSchedule, time.Now())
	if err != nil {
		log.Warn("payables: qualify failed: %v", err, c)
	}

	q := fee.Query(db)
	if s := c.Query("status"); s != "" {
		q = q.Filter("Status=", fee.Status(s))
	}
	if p := c.Query("program"); p != "" {
		q = q.Filter("Type=", fee.Type(p))
	}
	if payee := c.Query("payee"); payee != "" {
		q = q.Filter("PayeeId=", payee)
	}

	fees := make([]*fee.Fee, 0)
	if _, err := q.GetAll(&fees); err != nil {
		return http.Fail(c, 500, "failed to list payables", err)
	}
	paid, err := paidByFee(db)
	if err != nil {
		return http.Fail(c, 500, "failed to read recorded payments", err)
	}

	out := ListResponse{Payables: make([]Payable, 0, len(fees)), Matured: matured}
	byPayee := map[string]*group{}
	totals := map[currency.Type]money.Amount{}

	for _, f := range fees {
		if f.Status == fee.Refunded {
			continue
		}
		owed := f.Currency.Amount(f.Amount)
		settled, ok := paid[f.Id()]
		if !ok {
			settled = money.Zero(f.Currency.Money())
		}
		outstanding, err := owed.Sub(settled)
		if err != nil || outstanding.Sign() <= 0 {
			continue // fully settled
		}

		out.Payables = append(out.Payables, Payable{
			Id:          f.Id(),
			Name:        f.Name,
			Program:     f.Type,
			PayeeId:     f.PayeeId,
			Owed:        currency.Exact(owed),
			Paid:        currency.Exact(settled),
			Outstanding: currency.Exact(outstanding),
			Status:      f.Status,
			AccruedAt:   f.CreatedAt,
			EarnedBy:    f.PaymentId,
		})

		totals[f.Currency] = add(totals[f.Currency], outstanding)

		key := f.PayeeId + "|" + string(f.Currency)
		g, ok := byPayee[key]
		if !ok {
			g = &group{payee: f.PayeeId, program: f.Type}
			byPayee[key] = g
		}
		g.total = add(g.total, outstanding)
		g.count++
	}

	out.ByPayee = make([]PayeeTotal, 0, len(byPayee))
	for _, g := range byPayee {
		out.ByPayee = append(out.ByPayee, PayeeTotal{
			PayeeId:     g.payee,
			Program:     g.program,
			Outstanding: currency.Exact(g.total),
			Count:       g.count,
		})
	}
	out.Totals = make([]currency.Money, 0, len(totals))
	for _, a := range totals {
		out.Totals = append(out.Totals, currency.Exact(a))
	}

	return http.Render(c, 200, out)
}

type paymentRequest struct {
	Method    string         `json:"method"`    // eth | wire | other
	Reference string         `json:"reference"` // tx hash | wire ref | free text
	Settles   string         `json:"settles"`   // how much of the payable this clears, in ITS asset
	Sent      currency.Money `json:"sent"`      // what actually left, in whatever asset
	Note      string         `json:"note"`
}

// methods are the ways a human pays. They are one shape — method + reference +
// money + when + who — so there is one handler and no per-method branch.
var methods = map[transfer.Type]bool{transfer.ETH: true, transfer.Wire: true, transfer.Other: true}

// RecordPayment records that a human paid a payable out-of-band.
//
//	POST /v1/payables/:feeid/payments
//
// It moves no money and mutates no payable: it appends the fact. Recording the
// same (method, reference) twice returns the first record rather than settling
// twice, because a reference names one real-world payment.
func RecordPayment(c *zip.Ctx) error {
	if !middleware.RequirePlatformAdmin(c) {
		return nil
	}
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req paymentRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	m := transfer.Type(strings.ToLower(strings.TrimSpace(req.Method)))
	if !methods[m] {
		return http.Fail(c, 400, "method must be one of eth, wire, other", nil)
	}
	req.Reference = strings.TrimSpace(req.Reference)
	if req.Reference == "" {
		return http.Fail(c, 400, "reference is required — the tx hash, wire reference, or a description", nil)
	}

	// A reference names one real-world payment, so recording it twice settles once.
	prior := transfer.New(db)
	if _, found, err := transfer.Query(db).
		Filter("Type=", m).Filter("Reference=", req.Reference).First(prior); err != nil {
		return http.Fail(c, 500, "failed to check for an existing payment", err)
	} else if found {
		return http.Render(c, 200, prior)
	}

	fe := fee.New(db)
	if err := fe.GetById(c.Param("feeid")); err != nil {
		return http.Fail(c, 404, "payable not found", err)
	}
	if fe.Status == fee.Disputed {
		return http.Fail(c, 409, "payable is disputed", nil)
	}

	// The settlement is denominated in the payable's own asset — the caller does
	// not get to name one, so it can never disagree.
	settles, err := fe.Currency.ParseAmount(strings.TrimSpace(req.Settles))
	if err != nil {
		return http.Fail(c, 400, "settles is not a valid "+fe.Currency.Code()+" amount", err)
	}
	if settles.Sign() <= 0 {
		return http.Fail(c, 400, "settles must be positive", nil)
	}

	paid, err := paidByFee(db)
	if err != nil {
		return http.Fail(c, 500, "failed to read recorded payments", err)
	}
	settled, ok := paid[fe.Id()]
	if !ok {
		settled = money.Zero(fe.Currency.Money())
	}
	outstanding, err := fe.Currency.Amount(fe.Amount).Sub(settled)
	if err != nil {
		return http.Fail(c, 500, "failed to total what is owed", err)
	}
	if settles.Cmp(outstanding) > 0 {
		return http.Fail(c, 400, "amount exceeds the "+outstanding.Display()+" outstanding on this payable", nil)
	}

	// What actually left, exactly, at its own asset's scale — so wei survive and
	// never pass through a cents field.
	if !req.Sent.IsZero() {
		if _, err := req.Sent.Exact(); err != nil {
			return http.Fail(c, 400, "sent is not a valid "+req.Sent.Asset.Code()+" amount", err)
		}
	}

	tr := transfer.New(db)
	tr.FeeId = fe.Id()
	tr.PayeeId = fe.PayeeId
	tr.Settles = currency.Exact(settles)
	tr.Sent = req.Sent
	tr.Type = m
	tr.Reference = req.Reference
	tr.PaidAt = time.Now().UTC()
	tr.Actor = iammiddleware.GetIAMClaims(c).Subject
	tr.Note = req.Note

	if err := tr.Create(); err != nil {
		return http.Fail(c, 500, "failed to record the payment", err)
	}

	log.Info("payables: %s settled %s via %s ref=%s by %s", fe.Id(), settles.Display(), m, req.Reference, tr.Actor, c)
	return http.Render(c, 201, tr)
}
