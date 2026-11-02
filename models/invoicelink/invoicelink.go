// Package invoicelink resolves the link an invoice is shared by to the invoice
// it shows: the org whose books hold it and its id, in the system namespace,
// because the link's reader is anonymous and names no org.
//
// A link is a capability: 32 random bytes, encoded. The record is keyed by the
// SHA-256 of the link, never the link itself, so a reader of these records holds
// no working link.
package invoicelink

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[InvoiceLink]("invoice-link", orm.WithStringKey[InvoiceLink]())
}

// InvoiceLink is where the invoice a link shows lives.
type InvoiceLink struct {
	mixin.Model[InvoiceLink]

	Org     string `json:"org"`
	Invoice string `json:"invoice"`
	Test    bool   `json:"test,omitempty"`
}

func (l *InvoiceLink) Load(ps []datastore.Property) error  { return datastore.LoadStruct(l, ps) }
func (l *InvoiceLink) Save() ([]datastore.Property, error) { return datastore.SaveStruct(l) }

func db() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

// key is the record's id for a link: the hex SHA-256 of it.
func key(link string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(link)))
	return hex.EncodeToString(sum[:])
}

// New mints a link to invoice in org's books and records it.
func New(org, invoice string, test bool) (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	link := base64.RawURLEncoding.EncodeToString(b[:])
	d := db()
	l := new(InvoiceLink)
	l.Init(d)
	l.SetId(key(link))
	l.Org, l.Invoice, l.Test = org, invoice, test
	if err := l.Put(); err != nil {
		return "", err
	}
	return link, nil
}

// Find answers what link shows, and false when no link is recorded under it.
func Find(link string) (*InvoiceLink, bool, error) {
	if len(strings.TrimSpace(link)) < 32 {
		return nil, false, nil
	}
	d := db()
	l := new(InvoiceLink)
	l.Init(d)
	err := l.Get(d.NewKey(l.Kind(), key(link), 0, nil))
	switch {
	case err == nil:
		return l, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}
