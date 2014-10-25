package models

type User struct {
	id              string
	name            string
	email           string
	phone           string
	orders          []Order
	cart            Cart
	billingAddress  Address
	shippingAddress Address
}

type Address struct {
	street     string
	unit       string
	city       string
	state      string
	postalCode string
	country    string
}
