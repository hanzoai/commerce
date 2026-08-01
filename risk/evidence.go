// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/outcome"
	"github.com/hanzoai/commerce/models/paymentintent"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/payment/processor"
)

// ErrNoAdjudicator is the honest state of dispute SUBMISSION today: evidence
// can be assembled from our own record, and there is nowhere of ours to send it.
//
// Two routes exist in principle and neither is built. The processor that took
// the payment can forward evidence to the issuing network — that is
// [processor.DisputeProcessor], declared and implemented by no provider in this
// tree. A dispute NETWORK can stop a chargeback before it becomes one — and we
// are a member of none, so there is no such integration to call. A caller gets
// this error rather than a success that went nowhere.
var ErrNoAdjudicator = errors.New("risk: no adjudicator for this dispute — the processor cannot submit evidence and we belong to no dispute network")

// Evidence is a dispute defence assembled from the org's own record.
//
// It is two things at once, deliberately. Fields is the submission-ready form —
// the evidence field names an adjudicator understands, which is what
// [Submit] sends and what a merchant's own submission would carry. Packet is
// everything we know that bears on the charge, including the risk judgement that
// admitted it: the decision id, its score, the rules that hit and the controls in
// force. That second half is the part only we can produce, and it is the part
// that makes a defence more than a form.
type Evidence struct {
	Dispute   evidenceDispute         `json:"dispute"`
	Charge    *evidenceCharge         `json:"charge,omitempty"`
	Screens   []evidenceScreen        `json:"screens,omitempty"`
	Outcomes  []evidenceOutcome       `json:"outcomes,omitempty"`
	Fields    dispute.DisputeEvidence `json:"fields"`
	Assembled time.Time               `json:"assembled"`

	// Gaps names every fact a complete defence wants that this org's record does
	// not hold. It is stated rather than left blank because a merchant reading a
	// packet has to know what to add, and a blank field looks like a fact that
	// was checked and found empty.
	Gaps []string `json:"gaps,omitempty"`
}

type evidenceDispute struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	DueBy    string `json:"dueBy,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type evidenceCharge struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Customer string `json:"customer,omitempty"`
	Email    string `json:"email,omitempty"`
	Describe string `json:"describe,omitempty"`
	Provider string `json:"provider,omitempty"`
	Ref      string `json:"ref,omitempty"`
	At       string `json:"at,omitempty"`
}

type evidenceScreen struct {
	ID       string         `json:"id"`
	Stage    string         `json:"stage"`
	Action   string         `json:"action"`
	Score    float64        `json:"score,omitempty"`
	Agency   string         `json:"agency,omitempty"`
	Decision string         `json:"decision,omitempty"`
	Refusal  string         `json:"refusal,omitempty"`
	Detail   map[string]any `json:"detail,omitempty"`
	At       string         `json:"at,omitempty"`
}

type evidenceOutcome struct {
	ID    string `json:"id"`
	Event string `json:"event"`
	At    string `json:"at,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Assemble builds the defence for one dispute out of the org's own record. The
// datastore is namespaced to that org, so a dispute id from another tenant
// simply is not found — the tenant boundary is the store's, not a check this
// function performs and could forget.
func Assemble(db *datastore.Datastore, disputeID string) (*Evidence, error) {
	d := dispute.New(db)
	if err := d.GetById(disputeID); err != nil {
		return nil, err
	}

	e := &Evidence{
		Dispute: evidenceDispute{
			ID:       d.Id(),
			Amount:   d.Amount,
			Currency: string(d.Currency),
			Status:   string(d.Status),
			Reason:   d.Reason,
			Provider: d.ProviderRef,
		},
		Assembled: time.Now(),
	}
	if !d.EvidenceDueBy.IsZero() {
		e.Dispute.DueBy = d.EvidenceDueBy.UTC().Format(time.RFC3339)
	}

	// The charge, when the dispute names one.
	if d.PaymentIntentId == "" {
		e.Gaps = append(e.Gaps, "charge: the dispute names no payment intent")
	} else {
		pi := paymentintent.New(db)
		if err := pi.GetById(d.PaymentIntentId); err != nil {
			e.Gaps = append(e.Gaps, "charge: payment intent "+d.PaymentIntentId+" is not in this org's record")
		} else {
			e.Charge = &evidenceCharge{
				ID:       pi.Id(),
				Amount:   pi.Amount,
				Currency: string(pi.Currency),
				Status:   string(pi.Status),
				Customer: pi.CustomerId,
				Email:    pi.ReceiptEmail,
				Describe: pi.Description,
				Provider: pi.ProviderType,
				Ref:      pi.ProviderRef,
			}
			if at := pi.GetCreatedAt(); !at.IsZero() {
				e.Charge.At = at.UTC().Format(time.RFC3339)
			}
		}
	}

	// The judgement that admitted the charge, and every later judgement of the
	// same customer — the pattern is as much of the defence as the single event.
	for _, s := range screensFor(db, d, e.Charge) {
		row := evidenceScreen{
			ID: s.Id(), Stage: s.Stage, Action: s.Action, Score: s.Score,
			Agency: s.Agency, Decision: s.Decision, Refusal: s.Refusal, Detail: s.Detail,
		}
		if at := s.GetCreatedAt(); !at.IsZero() {
			row.At = at.UTC().Format(time.RFC3339)
		}
		e.Screens = append(e.Screens, row)
	}
	if len(e.Screens) == 0 {
		e.Gaps = append(e.Gaps, "judgement: this charge was never screened, so there is no decision to cite")
	}

	if e.Charge != nil && e.Charge.Customer != "" {
		for _, o := range outcome.For(db, KindCustomer, e.Charge.Customer) {
			row := evidenceOutcome{ID: o.Id(), Event: o.Event, Note: o.Note}
			if at := o.GetCreatedAt(); !at.IsZero() {
				row.At = at.UTC().Format(time.RFC3339)
			}
			e.Outcomes = append(e.Outcomes, row)
		}
	}

	e.Fields, e.Gaps = fields(d, e)
	return e, nil
}

// screensFor finds the risk record bearing on this dispute: the screen whose
// reference IS the charge, then every screen of that customer.
func screensFor(db *datastore.Datastore, d *dispute.Dispute, charge *evidenceCharge) []*screen.Screen {
	seen := map[string]bool{}
	out := []*screen.Screen{}

	add := func(rows []*screen.Screen) {
		for _, s := range rows {
			if seen[s.Id()] {
				continue
			}
			seen[s.Id()] = true
			out = append(out, s)
		}
	}

	if d.PaymentIntentId != "" {
		for _, s := range screen.For(db, "", "", 0) {
			if s.Reference == d.PaymentIntentId {
				add([]*screen.Screen{s})
			}
		}
	}
	if charge != nil && charge.Customer != "" {
		add(screen.For(db, KindCustomer, charge.Customer, 0))
	}
	return out
}

// fields renders the packet into the evidence field names an adjudicator reads,
// preferring evidence the merchant already supplied over anything derived: a
// merchant's own statement of what was sold is better than ours.
func fields(d *dispute.Dispute, e *Evidence) (dispute.DisputeEvidence, []string) {
	f := dispute.DisputeEvidence{}
	if d.Evidence != nil {
		f = *d.Evidence
	}
	gaps := e.Gaps

	if f.CustomerEmailAddress == "" && e.Charge != nil {
		f.CustomerEmailAddress = e.Charge.Email
	}
	if f.ProductDescription == "" && e.Charge != nil {
		f.ProductDescription = e.Charge.Describe
	}
	if f.ServiceDate == "" && e.Charge != nil {
		f.ServiceDate = e.Charge.At
	}
	if f.CustomerName == "" {
		gaps = append(gaps, "customerName: a charge does not record the cardholder's name; the merchant must supply it")
	}
	if f.UncategorizedText == "" {
		f.UncategorizedText = narrative(e)
	}
	return f, gaps
}

// narrative states, in the one free-text field every network accepts, what our
// own record says about this charge. It is assembled from facts and asserts
// nothing that is not in the packet above it.
func narrative(e *Evidence) string {
	var b strings.Builder
	if e.Charge != nil {
		fmt.Fprintf(&b, "Charge %s for %s %s", e.Charge.ID, minor(e.Charge.Amount), strings.ToUpper(e.Charge.Currency))
		if e.Charge.At != "" {
			fmt.Fprintf(&b, " on %s", e.Charge.At)
		}
		if e.Charge.Email != "" {
			fmt.Fprintf(&b, ", receipted to %s", e.Charge.Email)
		}
		b.WriteString(". ")
	}
	for _, s := range e.Screens {
		if s.Stage != string(Payment) {
			continue
		}
		fmt.Fprintf(&b, "Screened at authorization: %s", s.Action)
		if s.Decision != "" {
			fmt.Fprintf(&b, " (decision %s)", s.Decision)
		}
		if s.Agency != "" {
			fmt.Fprintf(&b, ", actor %s", s.Agency)
		}
		if s.Refusal != "" {
			fmt.Fprintf(&b, ", scoring refused: %s", s.Refusal)
		}
		b.WriteString(". ")
		break
	}
	return strings.TrimSpace(b.String())
}

// minor renders exact minor units as a decimal string without ever converting
// through a float. 1234 becomes "12.34".
func minor(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	s := strconv.FormatInt(cents/100, 10) + "." + fmt.Sprintf("%02d", cents%100)
	if neg {
		return "-" + s
	}
	return s
}

// Submit delivers an assembled defence to the adjudicator the org's processor
// reaches, and returns the receipt. reg is the org's processor registry.
//
// It fails with [ErrNoAdjudicator] when nothing can carry the packet — which is
// every case today. That refusal is the point: a merchant must not believe a
// defence was filed because a button returned 200.
func Submit(ctx context.Context, reg *processor.Registry, kind processor.ProcessorType, disputeRef string, e *Evidence) (*processor.DisputeReceipt, error) {
	if reg == nil || disputeRef == "" {
		return nil, ErrNoAdjudicator
	}
	dp, err := reg.GetDispute(kind)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoAdjudicator, err)
	}
	return dp.SubmitEvidence(ctx, disputeRef, map[string]string{
		"customer_name":          e.Fields.CustomerName,
		"customer_email_address": e.Fields.CustomerEmailAddress,
		"product_description":    e.Fields.ProductDescription,
		"service_date":           e.Fields.ServiceDate,
		"uncategorized_text":     e.Fields.UncategorizedText,
	})
}
