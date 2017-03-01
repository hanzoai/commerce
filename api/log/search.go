package log_

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/log"
	"hanzo.io/util/json/http"
)

type searchReq struct {
	After  time.Time `json:"after"`
	Before time.Time `json:"before"`
}

func search(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	req := &searchReq{}
	var lgs []*log_.Log

	q := log_.Query(db).Filter("Enabled=", true).Filter("Time>", req.After).Filter("Time<=", req.Before).Order("Time")
	if _, err := q.GetAll(&lgs); err != nil {
		http.Fail(c, 500, "Failed to get logs", err)
		return
	}

	http.Render(c, 200, lgs)
}
