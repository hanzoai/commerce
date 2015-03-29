package hashid

import (
	"errors"
	"fmt"
)

var kinds = map[string]int{
	"campaign2":     0,
	"collection2":   1,
	"coupon2":       2,
	"discounts2":    3,
	"order2":        4,
	"organization2": 5,
	"plan2":         6,
	"product2":      7,
	"token2":        8,
	"user2":         9,
	"variant2":      10,
	"payment":       11,
	"price2":        12,
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
