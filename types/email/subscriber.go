package email

import (
	"crypto/md5"
	"fmt"
	"io"

	. "hanzo.io/types"
)

type Subscriber struct {
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Email     Email    `json:"email"`
	Metadata  Map      `json:"metadata"`
	Tags      []string `json:"tags"`
}

func (s Subscriber) Md5() string {
	h := md5.New()
	io.WriteString(h, s.Email.Address)
	return fmt.Sprintf("%x", h.Sum(nil))
}
