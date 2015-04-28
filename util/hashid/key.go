package hashid

import (
	"errors"
	"strconv"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/models/constants"
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

type Namespace struct {
	IntId int64
	Name  string
}

func getRoot(ctx appengine.Context) *aeds.Key {
	return aeds.NewKey(ctx, "namespace", "", constants.NamespaceRootKey, nil)
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
	return getContext(ctx, constants.NamespaceNamespace)
}

// Get IntID by querying organization from it's namespace name
func getId(ctx appengine.Context, namespace string) int64 {
	if namespace == constants.NamespaceNamespace {
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
		return constants.NamespaceNamespace
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
	log.Debug("namespace: %v", namespace)

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

	log.Debug("encoded '%v' to %v", namespace, id)
	return int(id)
}

func decodeNamespace(ctx appengine.Context, encoded int) string {
	log.Debug("id: %v", encoded)
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

	log.Debug("decoded '%v' to %v", namespace, id)
	return namespace
}

func EncodeKey(ctx appengine.Context, key datastore.Key) string {
	log.Debug("key: %v", key)
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

	log.Debug("ids to encode: %v, %v", key, ids)
	return Encode(ids...)
}

func DecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key, err error) {
	log.Debug("encoded key: %v", encoded)

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
	ctx = getContext(ctx, namespace)

	// root key
	key = aeds.NewKey(ctx, decodeKind(ids[n-3]), "", int64(ids[n-2]), nil)

	// root key is always last key, so reverse through list to recreate key
	for i := n - 4; i >= 0; i = i - 2 {
		key = aeds.NewKey(ctx, decodeKind(ids[i-1]), "", int64(ids[i]), key)
	}

	return key, nil
}
