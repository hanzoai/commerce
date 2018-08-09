package subscriber

import (
	"crypto/md5"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/util/json"

	. "hanzo.io/types"
	. "hanzo.io/util/strings"
)

var mailchimpReserved = map[string]bool{
	"INTERESTS":  true,
	"REWARDS":    true,
	"ARCHIVE":    true,
	"USER_URL":   true,
	"DATE":       true,
	"EMAIL":      true,
	"EMAIL_TYPE": true,
	"TO":         true,
	"MC":         true,
	"LIST":       true,
}

var invalidFieldNameRe = regexp.MustCompile("[ -]")
var invalidNameChars = regexp.MustCompile(`[-_ ]`)
var invalidNameMorpheme = regexp.MustCompile(`^f|^l|^full|name$`)

func normalizeName(s string) string {
	empty := []byte("")
	b := invalidNameChars.ReplaceAll([]byte(s), empty)
	b = invalidNameMorpheme.ReplaceAll(b, empty)
	return string(b)
}

type Subscriber struct {
	mixin.Model

	Email  string `json:"email"`
	FormId string `json:"formId"`
	UserId string `json:"userId,omitempty"`

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

func (s Subscriber) Name() string {
	firstName := ""
	lastName := ""
	fullName := ""

	// Check metadata for name keys
	for k := range s.Metadata {
		k = normalizeName(k)
		if v, ok := s.Metadata[k].(string); ok {
			if k == "first" {
				firstName = v
			}
			if k == "last" {
				lastName = v
			}
			if k == "full" {
				fullName = v
			}

		}
	}

	// Use full name if found
	if fullName != "" {
		return fullName
	}

	// Combine first/last into full name
	parts := make([]string, 0)
	if firstName != "" {
		parts = append(parts, firstName)
	}

	if lastName != "" {
		parts = append(parts, lastName)
	}

	return strings.Join(parts, " ")
}

func (s Subscriber) MergeFields() map[string]interface{} {
	fields := make(map[string]interface{})

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

	// Normalize merge fields
	for k, v := range fields {
		// Remove any empty merge fields
		if v == "" {
			delete(fields, k)
		}

		// Rename invalid fieldnames
		if invalidFieldNameRe.Match([]byte(k)) {
			k2 := invalidFieldNameRe.ReplaceAll([]byte(k), []byte("_"))
			fields[string(k2)] = v
			delete(fields, k)
		}

		// Reserved field names should be renamed
		if _, ok := mailchimpReserved[strings.ToUpper(k)]; ok {
			fields["_"+k] = v
			delete(fields, k)
		}
	}

	return fields
}

func (s *Subscriber) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Subscriber) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	// Save properties
	return datastore.SaveStruct(s)
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
