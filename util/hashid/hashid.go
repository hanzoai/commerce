package hashid

import (
	"crowdstart.io/config"
	"github.com/speps/go-hashids"
)

var hd = hashids.NewData()

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
