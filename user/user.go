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
		if auth.IsLoggedIn(c, kind) {
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

			var orders []models.Order
			err = db.GetMulti(m.OrdersIds, orders)

			if err != nil {
				c.Fail(500, err)
				return
			}
			template.Render(c, "index.html", "orders", orders)
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
