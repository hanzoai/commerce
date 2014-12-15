package val

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/models"
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

func AjaxUser(c *gin.Context, user *models.User) bool {
	if !Check(user.FirstName).Exists().IsValid {
		log.Debug("Form posted without first name")
		c.JSON(400, gin.H{"message": "Please enter a first name."})
		return false
	}

	if !Check(user.LastName).Exists().IsValid {
		log.Debug("Form posted without last name")
		c.JSON(400, gin.H{"message": "Please enter a last name."})
		return false
	}

	if !Check(user.Phone).Exists().IsValid {
		log.Debug("Form posted without phone number")
		c.JSON(400, gin.H{"message": "Please enter a phone number."})
		return false
	}

	return true
}

func ValidateUser(c *gin.Context, user *models.User, errs []string) []string {
	if !Check(user.FirstName).Exists().IsValid {
		log.Debug("Form posted without first name")
		errs = append(errs, "Please enter a first name.")
	}

	if !Check(user.LastName).Exists().IsValid {
		log.Debug("Form posted without last name")
		errs = append(errs, "Please enter a last name.")
	}

	if !Check(user.Phone).Exists().IsValid {
		log.Debug("Form posted without phone number")
		errs = append(errs, "Please enter a phone number.")
	}

	return errs
}

func SanitizeUser(user *models.User) {
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.FirstName = strings.Title(user.FirstName)
	user.LastName = strings.Title(user.LastName)
}

func AjaxAddress(c *gin.Context, address *models.Address) bool {
	if !Check(address.Line1).Exists().IsValid {
		log.Debug("Form posted without address")
		c.JSON(400, gin.H{"message": "Please enter an address."})
		return false
	}

	if !Check(address.City).Exists().IsValid {
		log.Debug("Form posted without city")
		c.JSON(400, gin.H{"message": "Please enter a city."})
		return false
	}

	if !Check(address.State).Exists().IsValid {
		log.Debug("Form posted without state")
		c.JSON(400, gin.H{"message": "Please enter a state."})
		return false
	}

	if !Check(address.PostalCode).Exists().IsValid {
		log.Debug("Form posted without postal code")
		c.JSON(400, gin.H{"message": "Please enter a zip/postal code."})
		return false
	}

	if !Check(address.Country).Exists().IsValid {
		log.Debug("Form posted without country")
		c.JSON(400, gin.H{"message": "Please enter a country."})
		return false
	}

	return true
}

func ValidateAddress(c *gin.Context, address *models.Address, errs []string) []string {
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

func AjaxPassword(c *gin.Context, password *string) bool {
	if !Check(*password).IsPassword().IsValid {
		log.Debug("Form posted invalid password")
		c.JSON(400, gin.H{"message": "Password Must be atleast 6 characters long."})
		return false
	}

	return true
}
