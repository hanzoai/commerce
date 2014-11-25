package preorder

import (
	"crowdstart.io/auth"
	"crowdstart.io/models"
	"crowdstart.io/util/form"
	"github.com/gin-gonic/gin"
)

type PreorderForm struct {
	User            models.User
	Order           models.Order
	Password        string
	PasswordConfirm string
	ShippingAddress models.Address
	Token           models.InviteToken
}

func (f *PreorderForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	// Schema creates the Order.Items slice sized to whatever is the largest
	// index form item. This creates a slice with a huge number of nil structs,
	// so we create a new slice of items and use that instead.
	items := make([]models.LineItem, 0)
	for _, lineItem := range f.Order.Items {
		if lineItem.SKU() != "" {
			items = append(items, lineItem)
		}
	}
	f.Order.Items = items

	// Set password hash
	f.User.PasswordHash = auth.HashPassword(f.Password)

	return nil
}
