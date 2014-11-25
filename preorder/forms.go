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

	// For some reason gorilla/schema deserializes an extra nil lineItem,
	// we need to remove this.
	if f.Order.Items[0].SKU() == "" {
		slice := make([]models.LineItem, 0)
		f.Order.Items = append(slice, f.Order.Items[1:]...)
	}

	// Set password hash
	f.User.PasswordHash = auth.HashPassword(f.Password)

	return nil
}
