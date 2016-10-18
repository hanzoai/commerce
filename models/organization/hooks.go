package organization

// Hooks
func (o *Organization) BeforeCreate() error {
	o.Fees.Id = o.Id()
	return nil
}
