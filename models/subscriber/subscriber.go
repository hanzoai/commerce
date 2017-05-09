package subscriber

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"
	"time"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"

	. "hanzo.io/models"
	. "hanzo.io/util/strings"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Subscriber struct {
	mixin.Model

	Email         string `json:"email"`
	MailingListId string `json:"mailingListId"`
	UserId        string `json:"userId,omitempty"`

	Unsubscribed    bool      `json:"unsubscribed"`
	UnsubscribeDate time.Time `json:"unsubscribeDate,omitempty"`

	Client client.Client `json:"client"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s Subscriber) Md5() string {
	h := md5.New()
	io.WriteString(h, s.Email)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s Subscriber) MergeFields() map[string]string {
	fields := make(map[string]string)

	for k, v := range s.Metadata {
		fields[k] = fmt.Sprintf("%v", v)
	}

	// Update metadata with some extra client data
	fields["useragent"] = s.Client.UserAgent
	fields["referer"] = s.Client.Referer
	fields["language"] = s.Client.Language
	fields["country"] = s.Client.Country
	fields["region"] = s.Client.Region
	fields["city"] = s.Client.City

	// Remove any empty merge fields
	for k, v := range fields {
		if v == "" {
			delete(fields, k)
		}
	}

	return fields
}

func (s *Subscriber) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Subscriber) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Subscriber) Normalize() {
	s.Email = StripWhitespace(s.Email)
	s.Email = strings.ToLower(s.Email)
}

func FromJSON(db *datastore.Datastore, data []byte) *Subscriber {
	s := New(db)
	json.DecodeBytes(data, s)
	return s
}
