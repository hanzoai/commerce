package trigger

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/trigger/tasks"
	"hanzo.io/models/user"
	"hanzo.io/util/event"

	"hanzo.io/util/log"
)

var Tasks map[string]*delay.Function

type Action tasks.Action

type Checks map[string]interface{}

type Trigger struct {
	mixin.Model

	Name        string
	Action      Action `json:"action"`
	ActionArgs_ string `json:"-" datastore:",noindex"`

	Checks  Checks `json:"checks" datastore:"-"`
	Checks_ string `json:"-" datastore:",noindex"`
}

func (t *Trigger) Defaults() {
	t.Checks = make(Checks)
}

func (t Trigger) TryUser(event, orgId, userId string) {
	needed := len(t.Checks)
	for k, _ := range t.Checks {
		// Check Incoming Events
		if k == event {
			needed--
		}

		// Check Stored Tags
	}

	if needed != 0 {
		return
	}

	if tsk, ok := Tasks[t.Action.Task]; ok {
		tsk.Call(t.Db.Context, orgId, t.Action, userId)
	}
}

func init() {
	Tasks = tasks.Tasks

	event.Trigger = func(ctx context.Context, orgId string, event string, data interface{}) error {
		usr, ok := data.(*user.User)
		if !ok {
			return nil
		}
		log.Warn("TRIGGER %v", event)

		nsctx, _ := appengine.Namespace(ctx, orgId)
		db := datastore.New(nsctx)

		slice, err := Query(db).Filter("Checks."+event+"=", true).GetAll()
		if err != nil {
			return err
		}

		triggers, ok := slice.([]*Trigger)
		if !ok {
			return nil
		}

		for _, trigger := range triggers {
			trigger.TryUser(event, orgId, usr.Id())
		}

		return nil
	}
}
