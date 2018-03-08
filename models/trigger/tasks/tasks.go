package tasks

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models/segment"
	"hanzo.io/models/user"
	"hanzo.io/util/log"
)

type Action struct {
	Task string            `json:"task"`
	Args map[string]string `json:"args" datastore:"-"`
}

var Tasks = make(map[string]*delay.Function)

func AddTask(name string, fn interface{}) {
	Tasks[name] = delay.Func(name, fn)
}

func init() {
	AddTask("add-to-segment", func(ctx context.Context, orgId string, a Action, usrId string) {
		log.Warn("Call")

		nsCtx, err := appengine.Namespace(ctx, orgId)
		if err != nil {
			log.Error(err, ctx)
		}

		segId, ok := a.Args["SegmentId"]
		if !ok {
			log.Error("SegmentId Missing", ctx)
			return
		}

		db := datastore.New(nsCtx)
		usr := user.New(db)
		if err := usr.GetById(usrId); err != nil {
			log.Error("Could not find subscriber %v", usrId, ctx)
			return
		}

		seg := segment.New(db)
		if err := seg.GetById(segId); err != nil {
			log.Error("Could not find segment %v", segId, ctx)
			return
		}

		_, err = usr.GetOrCreateSub(segId)
		if err != nil {
			log.Error(err, ctx)
		}
	})
}
