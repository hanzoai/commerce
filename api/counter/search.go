package counter

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"

	aeds "google.golang.org/appengine/datastore"
)

type searchReq struct {
	Tag     string    `json:"tag"`
	StoreId string    `json:"storeId"`
	Geo     string    `json:geo`
	Period  string    `json:"period"`
	After   time.Time `json:"after"`
	Before  time.Time `json:"before"`
}

type searchRes struct {
	Count int `json:"count"`
}

func search(c *gin.Context) {
	req := searchReq{}
	if err := json.Decode(c.Request.Body, &req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	q := aeds.NewQuery(counter.ShardKind)

	if req.Before.IsZero() {
		req.Before = time.Now()
	}

	if req.Period == "" {
		req.Period = string(counter.None)
	}

	// Index Order Is Tag, StoreId, Period, Time, always query in this order
	q = q.Filter("Tag=", req.Tag).Filter("StoreId=", req.StoreId).Filter("Geo=", req.Geo)

	if req.Period == string(counter.Total) {
		q = q.Filter("Period=", req.Period)
	} else {
		q = q.Filter("Period=", req.Period).Filter("Time>", req.After).Filter("Time<=", req.Before)
	}

	shards := []counter.Shard{}

	ctx := middleware.GetAppEngine(c)

	res := searchRes{
		Count: 0,
	}

	log.Warn("Searching for %v", req, c)
	if _, err := q.GetAll(ctx, &shards); err != nil {
		log.Error("Counter Search Error %v", err, c)
	} else {
		log.Warn("Result Count %v", len(shards), c)
		for _, shard := range shards {
			res.Count += shard.Count
		}
	}

	http.Render(c, 200, res)
}
