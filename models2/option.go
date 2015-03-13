package models

type Option struct {
	// Ex. Size
	Name string
	// Ex. S M L
	Values []string
}

type VariantOption struct {
	// Ex. Size
	Name string
	// Ex. M
	Value string
}
