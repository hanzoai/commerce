package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.com/datastore"
	"crowdstart.com/models/aggregate"
	"crowdstart.com/models/analytics"
	"crowdstart.com/models/funnel"
	. "crowdstart.com/util/aggregate/tasks"
	"crowdstart.com/util/log"
)

var updateFunnels = delay.Func("UpdateFunnels", func(ctx appengine.Context, namespace, eventId string) {
	nsctx, err := appengine.Namespace(ctx, namespace)
	if err != nil {
		log.Error("Could not namespace %v, %v", namespace, err, ctx)
		return
	}

	db := datastore.New(nsctx)

	event := analytics.New(db)
	err = event.GetById(eventId)
	if err != nil {
		log.Error("Could not get event %v, %v", eventId, err, ctx)
		return
	}

	fs := make([]*funnel.Funnel, 0)
	_, err = funnel.Query(db).GetAll(fs)
	if err != nil {
		log.Error("Could not get funnel %v", err, ctx)
		return
	}

	// Loop over funnels
	for _, f := range fs {

		updateFunnel := false
		var counts = make([]int64, len(f.Events))
		// Loop over the events required by the funnel
		for i, step := range f.Events {
			found := false
			// Each step of the funnel must be a member of a set of events
			previousSameEvent := analytics.New(db)

			for _, option := range step {
				log.Debug("%v ?= %v", event.Name, option)
				if option == event.Name {
					// Get the last time this event happened, we want to track unique passes through the funnel
					if i > 0 {
						// Only if it is no the first event though (kind of pointless)
						previousSameEvent.Query().Filter("SessionId=", event.SessionId).Filter("Name=", option).Filter("CalculatedTimestamp<", event.CalculatedTimestamp).Order("-CalculatedTimestamp").Get()
					}
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
				previousStep := f.Events[last]
				for _, option := range previousStep {
					e := analytics.New(db)
					ok, err := e.Query().Filter("SessionId=", currentEvent.SessionId).Filter("Name=", option).Filter("CalculatedTimestamp>=", previousSameEvent.CalculatedTimestamp).Filter("CalculatedTimestamp<=", currentEvent.CalculatedTimestamp).Order("-CalculatedTimestamp").Get()
					if err != nil {
						log.Error("Could not get latest analytics event", err, ctx)
						return
					}

					if ok {
						log.Debug("%s ?= %s <? %s", option, e.Name, currentEvent.Name)
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
				counts[i] += 1
				updateFunnel = true
			}
		}
		if updateFunnel {
			UpsertAggregate(ctx, namespace, f.Name, "Funnel", event.CalculatedTimestamp, aggregate.Hourly, 0, counts)
		}
	}
})

func UpdateFunnels(ctx appengine.Context, namespace, eventId string) {
	updateFunnels.Call(ctx, namespace, eventId)
}
