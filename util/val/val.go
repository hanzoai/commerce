package val

import (
	"strings"

	"crowdstart.io/util/log"
)

type StringValidationContext struct {
	value   string
	IsValid bool
}

func Check(str string) *StringValidationContext {
	return &(StringValidationContext{str, true})
}

// Higher Level Validation
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
	s.IsValid = s.IsValid && (strings.Index(s.value, a) < strings.Index(s.value, b))
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
