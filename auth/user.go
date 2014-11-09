package auth

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/models"
	"crowdstart.io/datastore"
)

func GetUser(c *gin.Context) (user models.User, err error) {
	username, err := GetUsername(c)
	if err != nil {
		return user, err
	}

	db := datastore.New(c)
	err = db.Get(username, user)
	return user, err
}
