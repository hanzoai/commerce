package user

import (
	"crowdstart.io/auth"
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/router"
	"crowdstart.io/util/template"
	"errors"
	"github.com/gin-gonic/gin"
)

const kind = "user"

func init() {
	user := router.New("/user/")
	user.GET("/", func(c *gin.Context) {
		if auth.IsLoggedIn(c) {
			key, err := auth.GetKey(c, "User")
			if err != nil {
				c.Fail(500, err)
				return
			}

			db := datastore.New(c)
			m := new(models.User)
			err = db.GetKey("User", key, m)
			if err != nil {
				c.Fail(500, err)
				return
			}

			orders := make([]interface{}, len(m.OrdersIds))
			for i, v := range orders {
				orders[i] = interface{}(v)
			}

			err = db.GetMulti(m.OrdersIds, orders)

			o := make([]models.Order, len(orders))
			for i, v := range orders {
				o[i] = v.(models.Order)
			}

			if err != nil {
				c.Fail(500, err)
				return
			}
			template.Render(c, "index.html", "orders", o)
		}
	})

	user.POST("/login", func(c *gin.Context) {
		auth.VerifyUser(c, "user")
	})
}

func NewUser(c *gin.Context, f models.RegistrationForm) error {
	m := f.User
	db := datastore.New(c)
	q := db.Query(kind).
		Filter("Email =", m.Email).
		Limit(1)

	var admins [1]models.User
	_, err := q.GetAll(db.Context, &admins)

	if err != nil {
		return err
	}

	m.PasswordHash, err = f.PasswordHash()

	if err != nil {
		return err
	}

	if len(admins) == 1 {
		return errors.New("Email is already registered")
	} else {
		_, err := db.Put("admin", m)
		return err
	}
}
