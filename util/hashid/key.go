package hashid

import (
	"errors"
	"strconv"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
)

func encodeNamespace(namespace string) int {
	if namespace == "" {
		return 0
	}

	i, err := strconv.Atoi(namespace)
	if err != nil {
		panic(err)
	}
	return i
}

func decodeNamespace(namespace int) string {
	if namespace == 0 {
		return ""
	}

	return strconv.Itoa(namespace)
}

func EncodeKey(key datastore.Key) string {
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
		namespace = encodeNamespace(parent.Namespace())
	} else {
		namespace = encodeNamespace(key.Namespace())
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
	namespace := decodeNamespace(ids[n-1])
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
