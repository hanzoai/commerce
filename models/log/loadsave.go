package log

import (
	"strconv"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (l *Log) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	l.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(l, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(l.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(l.Metadata_), &l.Metadata)
	}

	if err != nil {
		return err
	}

	if len(l.Tags_) > 0 {
		err = json.DecodeBytes([]byte(l.Tags_), &l.Tags)
	}

	return err
}

func (l *Log) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	l.Metadata_ = string(json.EncodeBytes(&l.Metadata))
	l.Tags_ = string(json.EncodeBytes(&l.Tags))

	// Save properties
	props, err := datastore.SaveStruct(l)
	if err == nil {
		for k, v := range l.Tags {
			props = append(props, aeds.Property{Name: "Tags." + strconv.Itoa(k), Value: v})
		}
	}

	return props, err
}
