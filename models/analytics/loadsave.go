package analytics

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (e *AnalyticsEvent) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	e.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(e, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(e.Data_) > 0 {
		err = json.DecodeBytes([]byte(e.Data_), &e.Data)
	}

	return
}

func (e *AnalyticsEvent) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	e.Data_ = string(json.EncodeBytes(&e.Data))

	e.Name = e.Event
	if e.Event == "PageView" || e.Event == "PageLeave" {
		e.Name += "_" + e.PageId
	}

	// Save properties
	return datastore.SaveStruct(e)
}
