package hashid

import (
	"fmt"
	"time"

	"google.golang.org/appengine"
	aeds "google.golang.org/appengine/datastore"

	"github.com/qedus/nds"

	"hanzo.io/datastore/utils"
	"hanzo.io/models/namespace/consts"
	"hanzo.io/log"

	"hanzo.io/datastore/iface"
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
func getRoot(ctx context.Context) *aeds.Key {
	return aeds.NewKey(ctx, "namespace", "", consts.RootKey, nil)
}

// Query for namespace by Name
func queryNamespace(ctx context.Context, filter string, value interface{}) (*Namespace, bool, error) {
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
func getId(ctx context.Context, name string) int64 {
	if name == consts.Namespace {
		return 0
	}

	ns, ok, err := queryNamespace(ctx, "Name=", name)

	// Blow up if we can't complete query or find namespace
	if err != nil {
		panic(err.Error())
	}

	if !ok {
		panic(fmt.Errorf("Namespace '%s' does not exist", name))
	}

	return ns.IntId
}

// Get namespace from organization using it's IntID
func getName(ctx context.Context, id int64) (string, error) {
	if id == 0 {
		return consts.Namespace, nil
	}

	ns, ok, err := queryNamespace(ctx, "IntId=", id)

	// Query failed for some inexplicable reason
	if err != nil {
		return "", err
	}

	// Failed to find matching namespace
	if !ok {
		return "", fmt.Errorf("Namespace with id %d does not exist", id)
	}

	return ns.Name, nil
}

// Get namespaced context
func getContext(ctx context.Context, namespace string) context.Context {
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
func getNamespaceContext(ctx context.Context) context.Context {
	return getContext(ctx, consts.Namespace)
}

// Encodes organzation namespace into it's IntID
func encodeNamespace(ctx context.Context, namespace string) int {
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

func decodeNamespace(ctx context.Context, encoded int) (ns string, err error) {
	// Default namespace
	if encoded == 0 {
		return "", nil
	}

	id := int64(encoded)
	ns, ok := idToNamespace[id]
	if !ok {
		if ns, err = getName(ctx, id); err != nil {
			return "", err
		}

		// Cache result
		cache(ns, id)
	}

	return ns, nil
}

func EncodeKey(ctx context.Context, key iface.Key) string {
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

func DecodeKey(ctx context.Context, encoded string) (key *aeds.Key, err error) {
	ids, err := Decode(encoded)
	if err != nil {
		return nil, err
	}

	n := len(ids)

	// A valid key without parents will have exactly 3 segments: namespace,
	// kind and intid. For each parent we expect two more segments.
	if n < 3 || (n-3)%2 == 1 {
		return key, fmt.Errorf("Invalid number of segments: %v", ids)
	}

	// Set namespace
	ns, err := decodeNamespace(ctx, ids[n-1])
	if err != nil {
		return nil, err
	}

	ctx = getContext(ctx, ns)

	// root key
	kind, err := decodeKind(ids[n-3])
	if err != nil {
		return nil, err
	}
	key = aeds.NewKey(ctx, kind, "", int64(ids[n-2]), nil)

	// root key is always last key, so reverse through list to recreate key
	for i := n - 4; i >= 0; i = i - 2 {
		kind, err := decodeKind(ids[i-1])
		if err != nil {
			return nil, err
		}
		key = aeds.NewKey(ctx, kind, "", int64(ids[i]), key)
	}

	log.Debug("'%s' decoded to %s%v", encoded, fmtNs(ns), key)

	return key, nil
}

func MustDecodeKey(ctx context.Context, encoded string) (key *aeds.Key) {
	key, err := DecodeKey(ctx, encoded)
	if err != nil {
		panic(err)
	}

	return key
}

func KeyExists(ctx context.Context, encoded string) (bool, error) {
	key, err := DecodeKey(ctx, encoded)
	if err != nil {
		return false, err
	}

	// Try to query out matching key
	err = nds.Get(ctx, key, Model{})

	if err == aeds.ErrNoSuchEntity {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}
