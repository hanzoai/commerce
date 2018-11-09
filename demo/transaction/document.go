package transaction

import (
	"google.golang.org/appengine/search"

	"hanzo.io/models/mixin"
)

type Document struct {
	mixin.DocumentSaveLoad `datastore:"-" json:"-"`

	// Special Kind Facet
	Kind search.Atom `search:",facet"`

	Id_ string

	CreatedAt float64
	UpdatedAt float64

	Timestamp float64

	SendingAddress        string
	ReceivingAddress      string
	SendingName           string
	ReceivingName         string
	JuristictionSending   string
	JuristictionReceiving string
	Protocol              string
	TransactionHash       string
}

func (d Document) Id() string {
	return string(d.Id_)
}

func (d *Document) Init() {
	d.SetDocument(d)
}

func (t Transaction) Document() mixin.Document {

	doc := &Document{}
	doc.Init()
	doc.Kind = search.Atom(kind)
	doc.Id_ = t.Id()

	doc.CreatedAt = float64(t.CreatedAt.Unix())
	doc.UpdatedAt = float64(t.UpdatedAt.Unix())

	doc.SendingAddress = t.SendingAddress
	doc.ReceivingAddress = t.ReceivingAddress
	doc.SendingName = t.SendingName
	doc.ReceivingName = t.ReceivingName
	doc.JuristictionSending = t.JuristictionSending
	doc.JuristictionReceiving = t.JuristictionReceiving
	doc.Protocol = t.Protocol
	doc.TransactionHash = t.TransactionHash

	return doc
}
