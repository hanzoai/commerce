package processor

// Answer is a processor's own refusal of a call: the call reached it and it said
// no, so nothing it was asked to do was done. An error that is not an Answer
// may be a call that never got one, after which the processor may have acted.
type Answer struct{ Err error }

func (a *Answer) Error() string  { return a.Err.Error() }
func (a *Answer) Unwrap() error  { return a.Err }
func (a *Answer) Answered() bool { return true }

// ActionRequired is a processor's answer that the PAYER must act before the
// money can move — a bank's authentication step (3-D Secure). Nothing was taken.
// ClientSecret is what the browser's SDK needs to run the step; ID names the
// payment (or card setup) it belongs to, which the caller completes with once the
// payer has acted.
type ActionRequired struct {
	ID           string
	ClientSecret string
}

func (a *ActionRequired) Error() string {
	return "this card needs its bank's authentication before it can be charged"
}

// Answered is true: the processor said "not yet", and holds nothing.
func (a *ActionRequired) Answered() bool { return true }
