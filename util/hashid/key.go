package hashid

import (
	"errors"
	"strconv"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/namespace/consts"
	"crowdstart.com/util/log"
)

var (
	idToNamespace = make(map[int64]string)
	namespaceToId = make(map[string]int64)
)

func cache(namespace string, id int64) {
	idToNamespace[id] = namespace
	namespaceToId[namespace] = id
}

type Namespace struct {
	IntId int64
	Name  string
}

func getRoot(ctx appengine.Context) *aeds.Key {
	return aeds.NewKey(ctx, "namespace", "", consts.RootKey, nil)
}

func getContext(ctx appengine.Context, namespace string) appengine.Context {
	if namespace == "" {
		return ctx
	}

	ctx, err := appengine.Namespace(ctx, namespace)
	if err != nil {
		panic(err)
	}

	return ctx
}

func getNamespaceContext(ctx appengine.Context) appengine.Context {
	return getContext(ctx, consts.Namespace)
}

// Get IntID by querying organization from it's namespace name
func getId(ctx appengine.Context, namespace string) int64 {
	if namespace == consts.Namespace {
		return 0
	}

	ctx = getNamespaceContext(ctx)
	db := datastore.New(ctx)
	ns := Namespace{}

	// Use namespace root to ensure a strongly consistent query
	root := getRoot(ctx)
	_, ok, err := db.Query("namespace").Ancestor(root).Filter("Name=", namespace).First(&ns)
	err = datastore.IgnoreFieldMismatch(err)

	// Blow up if we can't find organization
	if err != nil {
		panic(err.Error())
	}
	if !ok {
		panic("Failed to retrieve namespace with Name: " + namespace)
	}

	return ns.IntId
}

// Get namespace from organization using it's IntID
func getNamespace(ctx appengine.Context, id int64) string {
	if id == 0 {
		return consts.Namespace
	}

	ctx = getNamespaceContext(ctx)
	db := datastore.New(ctx)
	ns := Namespace{}

	// Use namespace root to ensure a strongly consistent query
	root := getRoot(ctx)
	_, ok, err := db.Query("namespace").Ancestor(root).Filter("IntId=", id).First(&ns)
	err = datastore.IgnoreFieldMismatch(err)

	// Blow up if we can't find organization
	if err != nil {
		panic(err.Error())
	}
	if !ok {
		panic("Failed to retrieve namespace with Id: " + strconv.Itoa(int(id)))
	}

	return ns.Name
}

// Encodes organzation namespace into it's IntID
func encodeNamespace(ctx appengine.Context, namespace string) int {
	// Default namespace
	if namespace == "" {
		return 0
	}

	id, ok := namespaceToId[namespace]
	if !ok {
		id = getId(ctx, namespace)

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
		namespace = getNamespace(ctx, id)

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

	// Default to default namespace
	namespace := 0

	// Parent namespace overrides child
	if parent != nil {
		namespace = encodeNamespace(ctx, parent.Namespace())
	} else {
		namespace = encodeNamespace(ctx, key.Namespace())
	}

	// Append namespace
	ids = append(ids, namespace)

	encoded := Encode(ids...)

	log.Debug("%v encoded to '%s'", key, encoded)

	return encoded
}

func DecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key, err error) {
	// Catch panic from Decode
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case string:
				err = errors.New(v)
			case error:
				err = v
			default:
				err = errors.New("I don't even")
			}
		}
	}()

	ids := Decode(encoded)
	n := len(ids)

	// Check for invalid keys.
	if n < 3 {
		return key, errors.New("Invalid number of key segments")
	}

	// Set namespace
	namespace := decodeNamespace(ctx, ids[n-1])
	ctx = getContext(ctx, namespace)

	// root key
	key = aeds.NewKey(ctx, decodeKind(ids[n-3]), "", int64(ids[n-2]), nil)

	// root key is always last key, so reverse through list to recreate key
	for i := n - 4; i >= 0; i = i - 2 {
		key = aeds.NewKey(ctx, decodeKind(ids[i-1]), "", int64(ids[i]), key)
	}

	log.Debug("'%s' decoded to %v", encoded, key, namespace)

	return key, nil
}

func MustDecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key) {
	key, err := DecodeKey(ctx, encoded)
	if err != nil {
		panic(err)
	}

	return key
}
