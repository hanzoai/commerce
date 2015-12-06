package site

import (
	"crowdstart.com/config"
	"github.com/gin-gonic/gin"
)

func listFiles(c *gin.Context) {
}

func getFile(c *gin.Context) {
}

func putFile(c *gin.Context) {
	siteid := c.Param("siteid")
	deployid := c.Param("deployid")
	filepath := c.Param("filepath")

	url := "https://api.netlify.com/api/v1/sites/" + siteid + "/deploys/" + deployid + "/" + filepath
	url += "?access_token=" + config.Netlify.AccessToken
	c.Redirect(307, url)
}
