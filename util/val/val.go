package val

import (
	"strings"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

type Validator struct{}

func New() *Validator {
	return new(Validator)
}

type StringValidationContext struct {
	value   string
	IsValid bool
}

func Check(str string) *StringValidationContext {
	return &(StringValidationContext{str, true})
}

// Higher Level Validation
func (s *StringValidationContext) Empty() *StringValidationContext {
	return s.LengthIsGreaterThanOrEqualTo(1)
}

func (s *StringValidationContext) Exists() *StringValidationContext {
	return s.LengthIsGreaterThanOrEqualTo(1)
}

func (s *StringValidationContext) IsEmail() *StringValidationContext {
	return s.StringBeforeString("@", ".").LengthIsGreaterThanOrEqualTo(5) //a@b.c
}

func (s *StringValidationContext) IsPassword() *StringValidationContext {
	// We should ahve char restrictions here but whatever
	return s.LengthIsGreaterThanOrEqualTo(6) // Min Length 6
}

func (s *StringValidationContext) StringBeforeString(a string, b string) *StringValidationContext {
	s.Contains(a).Contains(b)
	s.IsValid = s.IsValid && (strings.Index(s.value, a) < strings.LastIndex(s.value, b))
	return s
}

// Basic Validation
func (s *StringValidationContext) LengthIsGreaterThanOrEqualTo(n int) *StringValidationContext {
	s.IsValid = s.IsValid && (len(s.value) >= n)
	log.Debug("%v >= %v is %v", s.value, n, s.IsValid)
	return s
}

func (s *StringValidationContext) EqualTo(str string) *StringValidationContext {
	s.IsValid = s.IsValid && (str == s.value)
	log.Debug("%v is equal to %v is %v", s.value, str, s.IsValid)
	return s
}

func (s *StringValidationContext) Contains(str string) *StringValidationContext {
	s.IsValid = s.IsValid && (strings.Contains(s.value, str))
	log.Debug("%v contains %v is %v", s.value, str, s.IsValid)
	return s
}

// func ValidateUser(user *models.User, errs []string) []string {
// 	if !Check(user.Email).IsEmail().IsValid {
// 		log.Debug("Form posted invalid email")
// 		errs = append(errs, "Please enter a valid email.")
// 	}

// 	if !Check(user.FirstName).Exists().IsValid {
// 		log.Debug("Form posted without first name")
// 		errs = append(errs, "Please enter a first name.")
// 	}

// 	if !Check(user.LastName).Exists().IsValid {
// 		log.Debug("Form posted without last name")
// 		errs = append(errs, "Please enter a last name.")
// 	}

// 	// Do we care?
// 	// if !Check(user.Phone).Exists().IsValid {
// 	// 	log.Debug("Form posted without phone number")
// 	// 	errs = append(errs, "Please enter a phone number.")
// 	// }

// 	return errs
// }

// func SanitizeUser(user *models.User) {
// 	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
// 	user.FirstName = strings.Title(user.FirstName)
// 	user.LastName = strings.Title(user.LastName)
// }

// func ValidateUser2(u *user.User, errs []string) []string {
// 	if !Check(u.Email).IsEmail().IsValid {
// 		log.Debug("Form posted invalid email")
// 		errs = append(errs, "Please enter a valid email.")
// 	}

// 	if !Check(u.FirstName).Exists().IsValid {
// 		log.Debug("Form posted without first name")
// 		errs = append(errs, "Please enter a first name.")
// 	}
// rrrrrr
// 	if !Check(u.LastName).Exists().IsValid {
// 		log.Debug("Form posted without last name")
// 		errs = append(errs, "Please enter a last name.")
// 	}

// 	// Do we care?
// 	// if !Check(u.Phone).Exists().IsValid {
// 	// 	log.Debug("Form posted without phone number")
// 	// 	errs = append(errs, "Please enter a phone number.")
// 	// }

// 	return errs
// }

// func SanitizeUser2(u *user.User) {
// 	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
// 	u.FirstName = strings.Title(u.FirstName)
// 	u.LastName = strings.Title(u.LastName)
// }

func ValidateAddress(address *models.Address, errs []string) []string {
	if !Check(address.Line1).Exists().IsValid {
		log.Debug("Form posted without address")
		errs = append(errs, "Please enter an address.")
	}

	if !Check(address.City).Exists().IsValid {
		log.Debug("Form posted without city")
		errs = append(errs, "Please enter a city.")
	}

	if !Check(address.State).Exists().IsValid {
		log.Debug("Form posted without state")
		errs = append(errs, "Please enter a state.")
	}

	if !Check(address.PostalCode).Exists().IsValid {
		log.Debug("Form posted without postal code")
		errs = append(errs, "Please enter a zip/postal code.")
	}

	if !Check(address.Country).Exists().IsValid {
		log.Debug("Form posted without country")
		errs = append(errs, "Please enter a country.")
	}
	return errs
}

func ValidatePassword(password string, errs []string) []string {
	if !Check(password).IsPassword().IsValid {
		log.Debug("Form posted invalid password")
		errs = append(errs, "Password Must be atleast 6 characters long.")
	}
	return errs
}
