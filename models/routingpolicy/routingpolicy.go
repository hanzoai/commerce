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
}

// Platform is the id of the row every org without its own inherits.
const Platform = "_platform"

// Row is one policy. Countries is the comma-joined ISO 3166-1 alpha-2 set Square
// takes cards in; it is a string because the store holds no lists.
type Row struct {
	mixin.Model[Row]

	SquareWeight int    `json:"squareWeight"`
	SquareOn     bool   `json:"squareOn"`
	StripeOn     bool   `json:"stripeOn"`
	LocalOn      bool   `json:"localOn"`
	PayPalOn     bool   `json:"paypalOn"`
	CryptoOn     bool   `json:"cryptoOn"`
	Countries    string `json:"countries"`
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
	r.SquareWeight, r.SquareOn, r.StripeOn, r.LocalOn, r.Countries, r.By = in.SquareWeight, in.SquareOn, in.StripeOn, in.LocalOn, in.Countries, in.By
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
