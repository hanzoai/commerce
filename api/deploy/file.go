package site

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/thirdparty/netlify"
)

func listFiles(c *gin.Context) {
}

func getFile(c *gin.Context) {
}

func putFile(c *gin.Context) {
	siteid := c.Param("siteid")
	deployid := c.Param("deployid")
	filepath := c.Param("filepath")

	ctx := middleware.GetAppEngine(c)
	org := middleware.GetOrganization(c)
	accessToken := netlify.GetAccessToken(ctx, org.Name)

	url := "https://api.netlify.com/api/v1/sites/" + siteid + "/deploys/" + deployid + "/" + filepath
	url += "?access_token=" + accessToken
	c.Redirect(307, url)
}
