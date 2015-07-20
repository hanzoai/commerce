package analytics

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
)

func js(c *gin.Context) {
	id := c.Params.ByName("organizationid")
	db := datastore.New(c)

	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, org.AnalyticsJs())
}
