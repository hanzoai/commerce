package hashid

import (
	"errors"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
)

var (
	idToNamespace = make(map[int64]string)
	namespaceToId = make(map[string]int64)
)

func cache(namespace string, id int64) {
	idToNamespace[id] = namespace
	namespaceToId[namespace] = id
}

type Organization struct {
	Name string
}

// Encodes organzation namespace into it's IntID
func encodeNamespace(ctx appengine.Context, namespace string) int {
	// Default namespace
	if namespace == "" {
		return 0
	}

	id, ok := namespaceToId[namespace]
	if !ok {
		// Lookup IntID
		ns, err := appengine.Namespace(ctx, "")
		if err != nil {
			panic(err)
		}
		db := datastore.New(ns)
		key, ok, err := db.Query2("organization").Filter("Name=", namespace).KeysOnly().First(nil)

		// Blow up if we can't find organization
		if err != nil {
			panic(err)
		}
		if !ok {
			panic("Failed to retrieve organization named: " + namespace)
		}

		// Get IntID
		id := key.IntID()

		// Cache result
		cache(namespace, id)
	}
	return int(id)
}

func decodeNamespace(ctx appengine.Context, encoded int) string {
	// Default namespace
	if encoded == 0 {
		return ""
	}

	id := int64(encoded)
	namespace, ok := idToNamespace[id]
	if !ok {
		// Lookup IntID
		ns, err := appengine.Namespace(ctx, "")
		if err != nil {
			panic(err)
		}

		db := datastore.New(ns)
		var org Organization
		key := db.NewKey("organization", "", id, nil)
		_, ok, err := db.Query2("organization").Filter("__key__=", key).Project("Name").First(&org)

		// Blow up if we can't find organization
		if err != nil {
			panic(err)
		}
		if !ok {
			panic("Failed to retrieve organization named: " + namespace)
		}

		// Get Namespace off organization
		namespace = org.Name

		// Cache result
		cache(namespace, id)
	}
	return namespace
}

func EncodeKey(ctx appengine.Context, key datastore.Key) string {
	id := int(key.IntID())

	// Return if incomplete key
	if id == 0 {
		return ""
	}

	ids := make([]int, 2)
	ids[0] = encodeKind(key.Kind())
	ids[1] = id

	// Add ancestor keys
	parent := key.Parent()
	for parent != nil {
		ids = append(ids, encodeKind(parent.Kind()), int(parent.IntID()))
		parent = parent.Parent()
	}

	// Use parent namespace if it exists, otherwise use child's
	namespace := 0

	// Parent namespace overrides child
	if parent != nil {
		namespace = encodeNamespace(ctx, parent.Namespace())
	} else {
		namespace = encodeNamespace(ctx, key.Namespace())
	}

	// Append namespace
	ids = append(ids, namespace)

	return Encode(ids...)
}

func DecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key, err error) {
	// Catch panic from Decode
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	ids := Decode(encoded)
	n := len(ids)

	// Check for invalid keys.
	if n < 3 {
		return key, errors.New("Invalid key")
	}

	// Set namespace
	namespace := decodeNamespace(ctx, ids[n-1])
	ctx, err = appengine.Namespace(ctx, namespace)
	if err != nil {
		return key, err
	}

	// root key
	key = aeds.NewKey(ctx, decodeKind(ids[n-3]), "", int64(ids[n-2]), nil)

	// root key is always last key, so reverse through list to recreate key
	for i := n - 4; i >= 0; i = i - 2 {
		key = aeds.NewKey(ctx, decodeKind(ids[i-1]), "", int64(ids[i]), key)
	}

	return key, nil
}
