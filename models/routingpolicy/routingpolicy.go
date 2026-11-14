// Package routingpolicy stores how an org's card checkouts are divided between
// processors, and the decisions made under it. One row per org, plus one
// platform row every org without its own inherits. Rows live in the system
// namespace, like pendingintent: the policy is the platform's, not a tenant's.
package routingpolicy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[Row]("routing-policy", orm.WithStringKey[Row]())
	orm.Register[Decision]("routing-decision", orm.WithStringKey[Decision]())
	orm.Register[Assignment]("routing-assignment", orm.WithStringKey[Assignment]())
}

// Platform is the id of the row every org without its own inherits.
const Platform = "_platform"

// Row is one policy.
type Row struct {
	mixin.Model[Row]

	SquareWeight int    `json:"squareWeight"`
	SquareOn     bool   `json:"squareOn"`
	StripeOn     bool   `json:"stripeOn"`
	LocalOn      bool   `json:"localOn"`
	PayPalOn     bool   `json:"paypalOn"`
	CryptoOn     bool   `json:"cryptoOn"`
	By           string `json:"by"`
}

func (r *Row) Load(ps []datastore.Property) error  { return datastore.LoadStruct(r, ps) }
func (r *Row) Save() ([]datastore.Property, error) { return datastore.SaveStruct(r) }

// Decision is one routing answer, kept so an operator can see where checkouts go.
type Decision struct {
	mixin.Model[Decision]

	Org       string `json:"org"`
	Processor string `json:"processor"`
	Country   string `json:"country"`
	Reason    string `json:"reason"`
}

func (d *Decision) Load(ps []datastore.Property) error  { return datastore.LoadStruct(d, ps) }
func (d *Decision) Save() ([]datastore.Property, error) { return datastore.SaveStruct(d) }

func db(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, "system"))
}

// Get answers the row stored under id (an org name or Platform).
func Get(ctx context.Context, id string) (*Row, bool, error) {
	d := db(ctx)
	r := new(Row)
	r.Init(d)
	err := r.Get(d.NewKey(r.Kind(), strings.ToLower(strings.TrimSpace(id)), 0, nil))
	switch {
	case err == nil:
		return r, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// Put stores row under id, replacing what was there.
func Put(ctx context.Context, id string, in Row) error {
	d := db(ctx)
	r := new(Row)
	r.Init(d)
	r.SetId(strings.ToLower(strings.TrimSpace(id)))
	r.SquareWeight, r.SquareOn, r.StripeOn, r.LocalOn, r.By = in.SquareWeight, in.SquareOn, in.StripeOn, in.LocalOn, in.By
	r.PayPalOn, r.CryptoOn = in.PayPalOn, in.CryptoOn
	return r.Put()
}

// Record keeps one routing answer.
func Record(ctx context.Context, org, processor, country, reason string) error {
	d := db(ctx)
	r := new(Decision)
	r.Init(d)
	var b [12]byte
	_, _ = rand.Read(b[:])
	r.SetId(hex.EncodeToString(b[:]))
	r.Org, r.Processor, r.Country, r.Reason = org, processor, country, reason
	return r.Put()
}

// Recent lists the latest decisions, newest first, at most n.
func Recent(ctx context.Context, n int) ([]*Decision, error) {
	rows := make([]*Decision, 0)
	if _, err := db(ctx).Query("routing-decision").GetAll(&rows); err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	// A decision older than two weeks has told the readout all it will: dropped as
	// they are read, a few at a time, so the table holds a fortnight and no more.
	dropped := 0
	for i := len(rows) - 1; i >= 0 && dropped < 200 && time.Since(rows[i].CreatedAt) > 14*24*time.Hour; i-- {
		_ = rows[i].Delete()
		rows = rows[:i]
		dropped++
	}
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}

// Assignment is the processor a payer was placed on, kept once so the split is a
// property of the customer and not of each checkout: a weight changed later moves
// nobody who has been placed. Attempt marks a Square charge begun and not yet known
// to have succeeded or failed.
type Assignment struct {
	mixin.Model[Assignment]

	Org       string    `json:"org"`
	Subject   string    `json:"subject"`
	Processor string    `json:"processor"`
	Attempt   string    `json:"attempt,omitempty"`
	AttemptAt time.Time `json:"attemptAt,omitempty"`
}

func (a *Assignment) Load(ps []datastore.Property) error  { return datastore.LoadStruct(a, ps) }
func (a *Assignment) Save() ([]datastore.Property, error) { return datastore.SaveStruct(a) }

func assignKey(org, subject string) string {
	return strings.ToLower(strings.TrimSpace(org)) + "|" + strings.ToLower(strings.TrimSpace(subject))
}

// Assigned answers where the payer was placed.
func Assigned(ctx context.Context, org, subject string) (*Assignment, bool, error) {
	d := db(ctx)
	a := new(Assignment)
	a.Init(d)
	err := a.Get(d.NewKey(a.Kind(), assignKey(org, subject), 0, nil))
	switch {
	case err == nil:
		return a, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// Place keeps processor as the payer's, replacing a placement that is no longer
// eligible. An open Attempt survives the replacement.
func Place(ctx context.Context, org, subject, processor string) error {
	cur, found, err := Assigned(ctx, org, subject)
	if err != nil {
		return err
	}
	d := db(ctx)
	a := new(Assignment)
	a.Init(d)
	a.SetId(assignKey(org, subject))
	a.Org, a.Subject, a.Processor = strings.ToLower(org), strings.ToLower(subject), processor
	if found {
		a.Attempt, a.AttemptAt = cur.Attempt, cur.AttemptAt
	}
	return a.Put()
}

// Begin marks a charge on processor started for the payer: until End says how it
// ended, no other processor is offered to them.
func Begin(ctx context.Context, org, subject, processor, ref string) error {
	cur, found, err := Assigned(ctx, org, subject)
	if err != nil {
		return err
	}
	d := db(ctx)
	a := new(Assignment)
	a.Init(d)
	a.SetId(assignKey(org, subject))
	a.Org, a.Subject, a.Processor = strings.ToLower(org), strings.ToLower(subject), processor
	if found && cur.Processor != "" {
		a.Processor = cur.Processor
	}
	a.Attempt, a.AttemptAt = ref, time.Now()
	return a.Put()
}

// End records that the charge begun under ref is known, whichever way it went.
func End(ctx context.Context, org, subject, ref string) error {
	cur, found, err := Assigned(ctx, org, subject)
	if err != nil || !found || cur.Attempt != ref {
		return err
	}
	cur.Attempt, cur.AttemptAt = "", time.Time{}
	return cur.Put()
}
