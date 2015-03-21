package accesstoken

import "github.com/gin-gonic/gin"

// Access token routes
func Get(c *gin.Context) {
	id := c.Params.ByName("id")

	query := c.Request.URL.Query()
	email := query.Get("email")
	password := query.Get("password")

	getAccessToken(c, id, email, password)
}

func Post(c *gin.Context) {
	id := c.Params.ByName("id")

	email := c.Request.Form.Get("email")
	password := c.Request.Form.Get("password")

	getAccessToken(c, id, email, password)
}

func Delete(c *gin.Context) {
	deleteAccessToken(c)
}
