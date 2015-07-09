package types

// These constraints are largely codes that come back as strings. For reference:
//   https://payments.amazon.com/documentation/apireference/201752850
//   https://payments.amazon.com/documentation/apireference/201752890
type Constraint struct {
	ConstraintID string // The identifier of the constraint.
	Description  string // Long-form description of the constraint.
}
