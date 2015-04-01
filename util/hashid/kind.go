package hashid

import (
	"errors"
	"fmt"
)

var kinds = map[string]int{
	"campaign":     0,
	"collection":   1,
	"coupon":       2,
	"discounts":    3,
	"order":        4,
	"organization": 5,
	"payment":      6,
	"plan":         7,
	"product":      8,
	"token":        9,
	"user":         10,
	"variant":      11,
	"price":        12,
}

var kindsReversed = make(map[int]string)

func init() {
	for k, v := range kinds {
		kindsReversed[v] = k
	}
}

func encodeKind(kind string) int {
	v, ok := kinds[kind]
	if ok {
		return v
	}
	err := errors.New(fmt.Sprintf("Unknown kind %v. Please register in util/hashid/kind.go.", kind))
	panic(err)

}

func decodeKind(encoded int) string {
	return kindsReversed[encoded]
}
