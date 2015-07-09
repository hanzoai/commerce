package types

type Destination struct {
	DestinationType     string  // it looks like this will always be 'physical'
	PhysicalDestination Address // ...yeah.
}
