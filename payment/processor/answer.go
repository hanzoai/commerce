package processor

// Answer is a processor's own refusal of a call: the call reached it and it said
// no, so nothing it was asked to do was done. An error that is not an Answer
// may be a call that never got one, after which the processor may have acted.
type Answer struct{ Err error }

func (a *Answer) Error() string  { return a.Err.Error() }
func (a *Answer) Unwrap() error  { return a.Err }
func (a *Answer) Answered() bool { return true }
