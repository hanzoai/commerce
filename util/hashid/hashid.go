package hashid

import (
	"errors"

	"appengine"

	"github.com/speps/go-hashids"

	"crowdstart.com/config"
)

var hd = hashids.NewData()

var MalformedHashId = errors.New("Hash id is malformed")

func init() {
	hd.Salt = config.Secret
	hd.MinLength = 10
}

func Encode(numbers ...int) string {
	h := hashids.NewWithData(hd)
	hashid, err := h.Encode(numbers)
	if err != nil {
		panic(err)
	}
	return hashid
}

func Decode(hashid string) []int {
	h := hashids.NewWithData(hd)
	return h.Decode(hashid)
}

func GetNamespace(ctx appengine.Context, hashid string) (string, error) {
	ids := Decode(hashid)
	// ids should never be empty...
	idsLen := len(ids)
	if idsLen <= 0 {
		return "", MalformedHashId
	}

	id := ids[idsLen-1]
	return decodeNamespace(ctx, id), nil
}
