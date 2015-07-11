package types

type Contact struct {
	Name                   Name    `json:"name"`                               // no api documentation
	Address                Address `json:"address"`                            // no api documentation
	PhoneNumber            string  `json:"phone_number,omitempty"`             // Phone number
	PhoneNumberAlternative string  `json:"phone_number_alternative,omitempty"` // Phone number alternative
	Email                  string  `json:"email,omitempty"`                    // Email address
}
