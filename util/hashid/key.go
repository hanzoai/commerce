package hashid

import (
	"errors"
	"fmt"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore/utils"
	"crowdstart.com/models/namespace/consts"
	"crowdstart.com/util/log"

	"crowdstart.com/datastore/iface"
)

var (
	idToNamespace = make(map[int64]string)
	namespaceToId = make(map[string]int64)

	IgnoreFieldMismatch = utils.IgnoreFieldMismatch
)

func cache(namespace string, id int64) {
	idToNamespace[id] = namespace
	namespaceToId[namespace] = id
}

type Model struct {
	Id_       string
	CreatedAt time.Time
	UpdatedAt time.Time
	Deleted   bool
}

type Namespace struct {
	// Included for compatibility with namespace models
	Model

	IntId int64
	Name  string
}

func fmtNs(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// Get root key for namespaces
func getRoot(ctx appengine.Context) *aeds.Key {
	return aeds.NewKey(ctx, "namespace", "", consts.RootKey, nil)
}

// Query for namespace by Name
func queryNamespace(ctx appengine.Context, filter string, value interface{}) (*Namespace, bool, error) {
	ns := new(Namespace)

	// Get namespaced context for namespaces
	ctx = getNamespaceContext(ctx)

	// Use namespace root to ensure a strongly consistent query
	root := getRoot(ctx)

	// Filter for namespace by name
	q := aeds.NewQuery("namespace").
		Ancestor(root).
		Filter(filter, value).
		Limit(1)

	// Run query
	key, err := q.Run(ctx).Next(ns)

	// Nothing found
	if key == nil {
		return nil, false, nil
	}

	// Error trying run query
	if err != nil {
		return nil, false, err
	}

	// Found it
	return ns, true, nil
}

// Get IntID for namespace
func getId(ctx appengine.Context, name string) int64 {
	if name == consts.Namespace {
		return 0
	}

	ns, ok, err := queryNamespace(ctx, "Name=", name)

	// Blow up if we can't find namespace
	if err != nil {
		panic(err.Error())
	}

	if !ok {
		panic(fmt.Sprintf("Namespace '%s' does not exists", name))
	}

	return ns.IntId
}

// Get namespace from organization using it's IntID
func getName(ctx appengine.Context, id int64) string {
	if id == 0 {
		return consts.Namespace
	}

	ns, ok, err := queryNamespace(ctx, "IntId=", id)

	// Blow up if we can't find namespace
	if err != nil {
		panic(err.Error())
	}

	if !ok {
		panic(fmt.Sprintf("Namespace with id %d does not exist", id))
	}

	return ns.Name
}

// Get namespaced context
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

// Get namespaced context for namespaces
func getNamespaceContext(ctx appengine.Context) appengine.Context {
	return getContext(ctx, consts.Namespace)
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
		namespace = getName(ctx, id)

		// Cache result
		cache(namespace, id)
	}

	return namespace
}

func EncodeKey(ctx appengine.Context, key iface.Key) string {
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

	log.Debug("%s%v encoded to '%s'", fmtNs(key.Namespace()), key, encoded)

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
				err = fmt.Errorf("Unknown panic decoding '%s'", encoded)
			}
		}
	}()

	ids := Decode(encoded)
	n := len(ids)

	// Check for invalid keys.
	if n < 3 {
		return key, fmt.Errorf("Invalid number of segments: %v", ids)
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

	log.Debug("'%s' decoded to %s%v", encoded, fmtNs(namespace), key)

	return key, nil
}

func MustDecodeKey(ctx appengine.Context, encoded string) (key *aeds.Key) {
	key, err := DecodeKey(ctx, encoded)
	if err != nil {
		panic(err)
	}

	return key
}

func KeyExists(ctx appengine.Context, encoded string) (bool, error) {
	key, err := DecodeKey(ctx, encoded)
	if err != nil {
		return false, err
	}

	// Try to query out matching key
	err = aeds.Get(ctx, key, Model{})

	if err == aeds.ErrNoSuchEntity {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}
