package analytics

import (
	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/analytics"
	"crowdstart.com/models/funnel"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/log"
)

var UpdateFunnels = delay.Func("UpdateFunnels", func(ctx appengine.Context, orgName, eventId string) {
	db := datastore.New(ctx)
	org := organization.New(db)

	err := org.GetById(orgName)
	if err != nil {
		log.Error("Could not get organization %v, %v", orgName, err, ctx)
		return
	}

	db = datastore.New(org.Namespace(ctx))

	event := analytics.New(db)
	err = event.GetById(eventId)
	if err != nil {
		log.Error("Could not get event %v, %v", eventId, err, ctx)
		return
	}

	var fs []*funnel.Funnel
	keys, err := funnel.Query(db).GetAll(&fs)
	if err != nil {
		log.Error("Could not get funnel %v", err, ctx)
		return
	}

	log.Warn("ORG %v", org)
	log.Warn("EVENT %v", event)
	log.Warn("FUNNELS %v", fs)

	// Loop over funnels
	for k, f := range fs {
		// Loop over the events required by the funnel
		for i, step := range f.Events {
			found := false
			// Each step of the funnel must be a member of a set of events
			for _, option := range step {
				log.Warn("%v ?= %v", event.Name, option)
				if option == event.Name {
					found = true
					break
				}
			}

			// If the event is in the set of the step of the current funnel, then validate
			if !found {
				// otherwise abort
				continue
			}

			currentEvent := event
			// Loop backwards over previous steps to see if we can find all the matching events
			last := i - 1
			for last >= 0 {
				found := false
				for _, option := range step {
					e := analytics.New(db)
					ok, err := e.Query().Filter("SessionId=", currentEvent.SessionId).Filter("Name=", option).Filter("CalculatedTimestamp<=", currentEvent.CalculatedTimestamp).Order("-CalculatedTimestamp").First()
					if err != nil {
						log.Error("Could not get latest analytics event", err, ctx)
						return
					}

					if ok {
						found = true
						currentEvent = e
						break
					}
				}

				// If the event is in the set of the previous steps, keep going...
				if !found {
					break
				}
				last--
			}

			// If the first element is reached, then update!
			if last == -1 {
				f.Counts[i]++
				f.Db = db
				f.Entity = f
				f.SetKey(keys[k])
				if err := f.Put(); err != nil {
					log.Error("Could not update funnel", err, ctx)
				}
			}
		}
	}
})
