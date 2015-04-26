package hashid

import (
	"errors"
	"io/ioutil"
	"strconv"

	"appengine"
	aeds "appengine/datastore"
	"appengine/urlfetch"

	"crowdstart.io/config"
	"crowdstart.io/datastore"
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

// Fetch namespace info from our API
func fetch(ctx appengine.Context, url string) (string, error) {
	// Get full URL to API endpoint
	endpoint := config.AbsoluteUrlFor("api", "/c/namespace")

	// Construct URL for request
	url = endpoint + url

	// Make request with urlfetch
	client := urlfetch.Client(ctx)
	res, err := client.Get(url)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", errors.New("API call failed")
	}

	// Return body
	body, err := ioutil.ReadAll(res.Body)
	return string(body), err
}

// Get IntID by querying organization from it's namespace name
func getId(ctx appengine.Context, namespace string) int64 {
	res, err := fetch(ctx, "/to-id/"+namespace)
	id, err := strconv.Atoi(res)
	if err != nil {
		log.Panic("Failed to retrieve id from namespace: %v", err, ctx)
	}
	return int64(id)
}

// Get namespace from organization using it's IntID
func getNamespace(ctx appengine.Context, id int64) string {
	res, err := fetch(ctx, "/by-id/"+strconv.Itoa(int(id)))
	if err != nil {
		log.Panic("Failed to retrieve namespace from id: %v", err, ctx)
	}
	return res
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
	// Default namespace
	if encoded == 0 {
		return ""
	}

	id := int64(encoded)
	namespace, ok := idToNamespace[id]
	if !ok {
		namespace := getNamespace(ctx, id)

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
