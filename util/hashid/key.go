package hashid

import (
	"errors"
	"fmt"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/util/context"
	"crowdstart.io/util/log"
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

func GetDefaultContext(ctx appengine.Context) appengine.Context {
	defaultCtx := context.Get(appengine.RequestID(ctx))
	if defaultCtx == nil {
		panic("Register Inflight Request to use DB functions")
	}

	return defaultCtx
}

// Get IntID by querying organization from it's namespace name
func getId(ctx appengine.Context, namespace string) int64 {
	db := datastore.New(GetDefaultContext(ctx))
	key, ok, err := db.Query2("organization").Filter("Name=", namespace).KeysOnly().First(nil)

	// Blow up if we can't find organization
	if err != nil {
		panic(err.Error())
	}
	if !ok {
		panic("Failed to retrieve organization named: " + namespace)
	}

	return key.IntID()
}

// Get namespace from organization using it's IntID
func getNamespace(ctx appengine.Context, id int64) string {
	db := datastore.New(GetDefaultContext(ctx))

	log.Warn("Decoding using id %v", id, ctx)

	var org Organization
	key := db.NewKey("organization", "", id, nil)
	err := datastore.IgnoreFieldMismatch(db.Get(key, &org))

	// Blow up if we can't find organization
	if err != nil {
		panic(fmt.Sprintf("Failed to retrieve organization with IntID: %v", id))
	}

	return org.Name
}

// Encodes organzation namespace into it's IntID
func encodeNamespace(ctx appengine.Context, namespace string) int {
	// Default namespace
	if namespace == "" {
		return 0
	}

	id, ok := namespaceToId[namespace]
	if !ok {
		id := getId(ctx, namespace)

		// Cache result
		cache(namespace, id)
	}
	return int(id)
}

func decodeNamespace(ctx appengine.Context, encoded int) string {
	log.Debug("Decoding a thing! %v", encoded)
	// Default namespace
	if encoded == 0 {
		return ""
	}

	id := int64(encoded)
	namespace, ok := idToNamespace[id]
	if !ok {
		namespace := getNamespace(ctx, id)

		log.Debug("Decoded a thing to %v, %v", namespace, id)
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

	log.Debug("Encoding keyyyyyy: %v, %v", key, ids)

	return Encode(ids...)
}

func DecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key, err error) {
	log.Debug("Decoding key: %v", encoded)
	// Catch panic from Decode
	defer func() {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
			log.Warn("Failed to decode key '%v': %v", encoded, err, ctx)
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
