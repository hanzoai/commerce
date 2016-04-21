package hashid

import (
	"errors"

	"crowdstart.com/config"
	"github.com/speps/go-hashids"

	"appengine"
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

func Decode(hashid string) ([]int, error) {
	var err error

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

	h := hashids.NewWithData(hd)
	return h.Decode(hashid), err
}

func GetNamespace(ctx appengine.Context, hashid string) (string, error) {
	ids, err := Decode(hashid)
	if err != nil {
		return "", err
	}
	// ids should never be empty...
	idsLen := len(ids)
	if idsLen <= 0 {
		return "", MalformedHashId
	}

	id := ids[idsLen-1]
	return decodeNamespace(ctx, id), nil
}
