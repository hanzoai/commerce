package user

import (
	"github.com/gin-gonic/gin"

	// "crowdstart.com/models"
	"crowdstart.com/util/form"
	"crowdstart.com/util/val"

	"crowdstart.com/models/user"

	. "crowdstart.com/models"
)

// User profile form (contact)
type ContactForm struct {
	User *user.User
}

func (f *ContactForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *ContactForm) Validate() []string {
	var errs []string
	// errs = val.ValidateUser(&f.User, errs)
	return errs
}

// User profile form (billing)
type BillingForm struct {
	BillingAddress Address
}

func (f *BillingForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *BillingForm) Validate() []string {
	var errs []string
	// errs = val.ValidateAddress(&f.BillingAddress, errs)
	return errs
}

// User profile form (change password)
type ChangePasswordForm struct {
	OldPassword     string
	Password        string
	ConfirmPassword string
}

func (f *ChangePasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

func (f *ChangePasswordForm) Validate() []string {
	var errs []string
	errs = val.ValidatePassword(f.Password, errs)
	return errs
}

// Reset Password form (request)
type ResetPasswordForm struct {
	Email string
}

func (f *ResetPasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

// Reset Password form (confirm)
type ResetPasswordConfirmForm struct {
	NewPassword     string
	ConfirmPassword string
}

func (f *ResetPasswordConfirmForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}

// // User profile form (metadata)
// type MetadataForm struct {
// 	Metadata []models.Datum // Not on HTML form directly; generated when parsed
// }

// func (f *MetadataForm) Parse(c *gin.Context) error {
// 	if err := c.Request.ParseForm(); err != nil {
// 		return err
// 	}

// 	// Create Metadata from the HTML 'name' element and the values in the inputs.
// 	for key, value := range c.Request.Form {
// 		keyExists := false
// 		// Range over the existing metadata to see if we can find a matching key.
// 		for datumkey, datum := range f.Metadata {
// 			if datum.Key == key {
// 				// We found a matching key. Note we did and update the existing value.
// 				f.Metadata[datumkey].Value = value[0]
// 				keyExists = true
// 				break
// 			}
// 		}
// 		if keyExists == false {
// 			// We didn't find a datum with the key we were looking for.  So create one and append it.
// 			f.Metadata = append(f.Metadata, models.Datum{Key: key, Value: value[0]})
// 		}
// 	}
// 	return nil
// }

// func (f *MetadataForm) Validate() []string {
// 	var errs []string
// 	return errs
// }
