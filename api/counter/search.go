package counter

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/counter"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	aeds "appengine/datastore"
)

type searchReq struct {
	Tag     string    `json:"tag"`
	Period  string    `json:"period"`
	StoreId string    `json:"storeId"`
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

	q = q.Filter("Tag=", req.Tag)

	if req.Period == string(counter.Total) {
		q = q.Filter("Period=", req.Period)
	} else {
		q = q.Filter("Time>", req.After).Filter("Time<=", req.Before).Filter("StoreId=", req.StoreId).Filter("Period=", req.Period)
	}

	shards := []counter.Shard{}

	ctx := middleware.GetAppEngine(c)

	res := searchRes{
		Count: 0,
	}

	if _, err := q.GetAll(ctx, &shards); err != nil {
		log.Error("Counter Search Error %v", err, c)
	} else {
		for _, shard := range shards {
			res.Count += shard.Count
		}
	}

	http.Render(c, 200, res)
}
