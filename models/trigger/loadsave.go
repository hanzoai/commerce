package trigger

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (t *Trigger) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	t.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(t, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(t.Checks_) > 0 {
		err = json.DecodeBytes([]byte(t.Checks_), &t.Checks)
	}

	// Deserialize action from datastore
	if len(t.ActionArgs_) > 0 {
		err = json.DecodeBytes([]byte(t.ActionArgs_), &t.Action.Args)
	}

	return err
}

func (t *Trigger) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	t.Checks_ = string(json.EncodeBytes(&t.Checks))

	// Serialize action
	t.ActionArgs_ = string(json.EncodeBytes(&t.Action.Args))

	// Save properties
	props, err := datastore.SaveStruct(t)
	if err == nil {
		for k, v := range t.Checks {
			props = append(props, aeds.Property{Name: "Checks." + k, Value: v})
		}
	}

	return props, err
}
