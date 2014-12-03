package user

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"errors"
	"github.com/gin-gonic/gin"
)

const kind = "user"

func Login(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		// Success
	} else {
		c.Fail(401, err)
	}
}

func DisplayOrders(c *gin.Context) {
	key, err := auth.Get(c, "login-key")
	if err != nil {
		c.Fail(500, err)
		return
	}

	user := auth.GetUser(c)
	if user == nil {
		log.Panic("User was not found")
	}

	orders := make([]interface{}, len(m.OrdersIds))
	for i, v := range orders {
		orders[i] = interface{}(v)
	}

	err = db.GetMulti(m.OrdersIds, orders)
	if err != nil {
		log.Panic("Error while retrieving orders", err)
	}

	o := make([]models.Order, len(orders))
	for i, v := range orders {
		o[i] = v.(models.Order)
	}

	template.Render(c, "index.html", "orders", o)
}

func NewUser(c *gin.Context, f auth.RegistrationForm) error {
	m := f.User
	db := datastore.New(c)

	user := new(models.User)
	db.GetKey("user", m.Email, user)
	if user == nil {
		m.PasswordHash, _ = f.PasswordHash()
		_, err := db.Put("user", m)
		return err
	}

	return errors.New("Email is already registered")
}