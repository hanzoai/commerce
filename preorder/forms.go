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
	HelmetColor     string `"schema":-`
	HelmetSize      string `"schema":-`
	HelmetQuantity  string `"schema":-`
	ShirtStyle      string `"schema":-`
	ShirtColor      string `"schema":-`
	ShirtSize       string `"schema":-`
	ShirtQuanity    string `"schema":-`
}

func (f *PreorderForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		return err
	}

	f.User.PasswordHash = auth.HashPassword(f.Password)

	return nil
}
