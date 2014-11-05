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

func init() {
	user := router.New("/user/")
	user.GET("/", func(c *gin.Context) {
		template.Render(c, "index.html")
	})

	user.POST("/login", func(c *gin.Context) {
		auth.VerifyUser(c, "user")
	})
}

func NewUser(c *gin.Context, f models.RegistrationForm) error {
	m := f.User
	db := datastore.New(c)
	q := db.Query("user").
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
