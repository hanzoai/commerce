package user

import (
	"strings"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (u *User) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	u.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(u, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(u.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(u.Metadata_), &u.Metadata)
	}

	return
}

func (u *User) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	u.Metadata_ = string(json.EncodeBytes(&u.Metadata))

	// sanitize email
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))

	// Save properties
	return datastore.SaveStruct(u)
}
