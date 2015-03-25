package val

import (
	"fmt"
	"reflect"
	"strings"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

type ValidatorFunction func(interface{}) *FieldError

// Storage for custom rules
var customRules map[string]ValidatorFunction

// RegisterRule adds a custom rule to the validaiton library
func RegisterRule(name string, vfn ValidatorFunction) {
	customRules[name] = vfn
}

type Validator struct {
	Value     interface{}
	lastField string
	fnsMap    map[string][]ValidatorFunction
}

// Create a new Validator using a custom struct
func New(value interface{}) *Validator {
	return &Validator{Value: value}
}

// Helper to dereference all pointer layers
func depointer(value reflect.Value) reflect.Value {
	// Strip off all the pointers
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value
}

// Set the current field to be Validated
func (v *Validator) Check(field string) *Validator {
	// we have to add the & to make the fields on the value settable
	rawValue := depointer(reflect.ValueOf(&v.Value))
	if !rawValue.FieldByName(field).IsValid() {
		log.Panic("Field does not exist!")
	}

	if _, ok := v.fnsMap[field]; !ok {
		v.fnsMap[field] = make([]ValidatorFunction, 0)
	}
	return v
}

func (v *Validator) Execute() []*FieldError {
	structValue := depointer(reflect.ValueOf(&v.Value))
	var i interface{}
	errs := make([]*FieldError, 0)

	for field, fns := range v.fnsMap {
		switch value := structValue.FieldByName(field); value.Kind() {
		case reflect.Bool:
			i = value.Bool()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i = value.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			i = value.Uint()
		case reflect.String:
			i = value.String()
		// case reflect.Slice:
		// 	// don't handle
		// case reflect.Complex64, reflect.Complex128:
		// 	// don't handle
		case reflect.Float32, reflect.Float64:
			i = value.Float()
		default:
			log.Panic("Validator does not support type '%v'", value.Type())
		}
		for _, fn := range fns {
			if err := fn(i); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

func (v *Validator) Exists() *Validator {
	field := v.lastField
	v.fnsMap[field] = append(v.fnsMap[field], func(i interface{}) *FieldError {
		switch value := i.(type) {
		case string:
			if len(value) > 0 {
				return nil
			}
		}
		return NewFieldError(field, "Field cannot be blank")
	})
	return v
}

func (v *Validator) IsEmail() *Validator {
	field := v.lastField
	v.fnsMap[field] = append(v.fnsMap[field], func(i interface{}) *FieldError {
		switch value := i.(type) {
		case string:
			if strings.Contains(value, "@") &&
				strings.Contains(value, ".") &&
				strings.Index(value, "@") < strings.Index(value, ".") &&
				len(value) > 5 {
				return nil
			}
		}
		return NewFieldError(field, "Email is invalid")
	})
	return v
}

func (v *Validator) IsPassword() *Validator {
	field := v.lastField
	v.fnsMap[field] = append(v.fnsMap[field], func(i interface{}) *FieldError {
		switch value := i.(type) {
		case string:
			if len(value) >= 6 {
				return nil
			}
		}
		return NewFieldError(field, "Passwords must be atleast 6 characters long.")
	})
	return v
}

func (v *Validator) MinLength(minLength int) *Validator {
	field := v.lastField
	v.fnsMap[field] = append(v.fnsMap[field], func(i interface{}) *FieldError {
		switch value := i.(type) {
		case string:
			if len(value) >= minLength {
				return nil
			}
		}
		return NewFieldError(field, fmt.Sprintf("Field must be atleast %d characters long.", minLength))
	})
	return v
}

// Use for enums
func (v *Validator) Matches(strs ...string) *Validator {
	field := v.lastField
	v.fnsMap[field] = append(v.fnsMap[field], func(i interface{}) *FieldError {
		switch value := i.(type) {
		case string:
			for _, str := range strs {
				if str == value {
					return nil
				}
			}
		}
		return NewFieldError(field, fmt.Sprintf("Field must be match one of ['%v'].", strings.Join(strs, "', '")))
	})
	return v
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
