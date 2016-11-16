package mailchimp

import "github.com/zeekay/gochimp3"

type Error struct {
	*gochimp3.APIError
}

func newError(err error) *Error {
	if aerr, ok := err.(*gochimp3.APIError); ok {
		return &Error{aerr}
	}
	return &Error{nil}
}
