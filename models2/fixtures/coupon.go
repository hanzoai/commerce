package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/models2/coupon"
)

func Coupon(c *gin.Context) *coupon.Coupon {
	db := getDb(c)

	// Owner for this organization
	user := user.New(db)
	user.Email = "dev@hanzo.ai"
	user.GetOrCreate("Email=", user.Email)

	user.FirstName = "Jackson"
	user.LastName = "Shirts"
	user.Phone = "(999) 999-9999"
	user.PasswordHash = auth.HashPassword("suchtees")
	user.MustPut()
	return user
}
