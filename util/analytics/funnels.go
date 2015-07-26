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

var UpdateFunnels = delay.Func("UpdateFunnels", func(ctx appengine.Context, org *organization.Organization, event *analytics.AnalyticsEvent) {
	nsctx := org.Namespace(ctx)
	db := datastore.New(nsctx)

	var fs []*funnel.Funnel
	_, err := analytics.Query(db).GetAll(&fs)
	if err != nil {
		log.Error("Could not get funnel", err, ctx)
		return
	}

	// Loop over funnels
	for _, f := range fs {
		// Loop over the events required by the funnel
		for i, step := range f.Events {
			found := false
			// Each step of the funnel must be a member of a set of events
			for _, option := range step {
				if option == event.Name {
					found = true
					break
				}
			}

			// If the event is in the set of the step of the current funnel, then validate
			if !found {
				// otherwise abort
				break
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

				// ...until reaching the first element
				if last == 0 {
					f.Counts[i]++
					if err := f.Put(); err != nil {
						log.Error("Could not update funnel", err, ctx)
					}
				}
				last--
			}
		}

	}
})
