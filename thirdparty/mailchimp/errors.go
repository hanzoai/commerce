package mailchimp

import "github.com/zeekay/gochimp3"

type Error struct {
	Unknown   error
	Mailchimp *gochimp3.APIError
	Status    int
}

func (e *Error) Error() string {
	if e.Mailchimp != nil {
		return e.Mailchimp.Error()
	}
	return e.Unknown.Error()
}

func wrapError(fn func() error) *Error {
	// Do work
	err := fn()

	// Not an error
	if err == nil {
		return nil
	}

	// Handle Mailchimp API Errors
	if merr, ok := err.(*gochimp3.APIError); ok {
		return &Error{Mailchimp: merr, Status: merr.Status}
	}

	// Handle any other errors
	return &Error{Unknown: err, Status: 500}
}
