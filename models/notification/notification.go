package notification

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type Channel string

const (
	Email Channel = "email"
	SMS   Channel = "sms"
	Push  Channel = "push"
)

type NotificationStatus string

const (
	Pending NotificationStatus = "pending"
	Sent    NotificationStatus = "sent"
	Failed  NotificationStatus = "failed"
)

type Notification struct {
	mixin.Model

	To         string             `json:"to"`
	Channel    Channel            `json:"channel"`
	TemplateId string             `json:"templateId"`
	Status     NotificationStatus `json:"status"`
	ProviderId string             `json:"providerId"`
	ExternalId string             `json:"externalId"`

	// Data stored as JSON in datastore
	Data  Map    `json:"data,omitempty" datastore:"-"`
	Data_ string `json:"-" datastore:",noindex"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (n *Notification) Load(ps []datastore.Property) (err error) {
	n.Defaults()

	if err = datastore.LoadStruct(n, ps); err != nil {
		return err
	}

	if len(n.Data_) > 0 {
		if err = json.DecodeBytes([]byte(n.Data_), &n.Data); err != nil {
			return err
		}
	}

	if len(n.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(n.Metadata_), &n.Metadata)
	}

	return err
}

func (n *Notification) Save() ([]datastore.Property, error) {
	n.Data_ = string(json.EncodeBytes(&n.Data))
	n.Metadata_ = string(json.EncodeBytes(&n.Metadata))

	return datastore.SaveStruct(n)
}
