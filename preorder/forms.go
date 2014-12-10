package preorder

import (
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
)

type PreorderForm struct {
	User            models.User
	Orders          []models.Order
	Password        string
	PasswordConfirm string
	ShippingAddress models.Address
	Token           models.InviteToken
}

func (f *PreorderForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// // Checks if the both passwords on the form are equal
	// if f.Password != f.PasswordConfirm {
	// 	return errors.New("Password and password confirmation are not equal")
	// }

	// // And if the password is at least 6 chars long
	// if len(f.Password) < 6 {
	// 	return errors.New("Password is less than 6 characters long")
	// }

	// removes whitespace
	f.User.Email = strings.TrimSpace(f.User.Email)

	// Schema creates the Order.Items slice sized to whatever is the largest
	// index form item. This creates a slice with a huge number of nil structs,
	// so we create a new slice of items and use that instead.

	for i, order := range f.Orders {
		var items []models.LineItem
		for _, item := range order.Items {
			if item.SKU() != "" {
				items = append(items, item)
			}
		}
		f.Orders[i].Items = items
	}

	// Set password hash
	f.User.PasswordHash = auth.HashPassword(f.Password)

	return nil
}
