package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/util/template"
)

const kind = "user"

func Login(c *gin.Context) {
	if err := auth.VerifyUser(c); err == nil {
		c.Redirect(300, "/user/")
	} else {
		template.Render(c, "platform/user/login.html",
			"error", "Invalid email or password",
		)
	}
}
