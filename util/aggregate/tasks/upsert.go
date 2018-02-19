package tasks

import (
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"

	"hanzo.io/datastore"
	"hanzo.io/models/aggregate"
	"hanzo.io/log"
)

var upsertAggregate = delay.Func("UpsertAggregate", func(ctx context.Context, namespace, name, typ string, t time.Time, f string, deltaValue int, deltaVectorValue []int64) {
	freq := aggregate.Frequency(f)

	nsctx, err := appengine.Namespace(ctx, namespace)
	if err != nil {
		log.Error("Could not namespace %v, %v", namespace, err, ctx)
		return
	}

	db := datastore.New(nsctx)
	err = db.RunInTransaction(func(db *datastore.Datastore) error {
		agg := aggregate.New(db)
		aggregate.Init(agg, name, t, freq)

		if err := agg.GetById(agg.Instance); err != nil {
			// insert aggregate
			agg.Value = int64(deltaValue)
			agg.VectorValue = deltaVectorValue
		} else {
			// update aggregate
			agg.Value += agg.Value

			if deltaVectorValue != nil {
				if agg.VectorValue == nil {
					agg.VectorValue = deltaVectorValue
				} else {
					for len(deltaVectorValue) > len(agg.VectorValue) {
						agg.VectorValue = append(agg.VectorValue, 0)
					}

					for i, v := range deltaVectorValue {
						agg.VectorValue[i] += v
					}
				}
			}
		}

		agg.Type = typ
		if err := agg.Put(); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		// Poor man's retry
		panic(err)
	}
})

func UpsertAggregate(ctx context.Context, namespace, name, typ string, t time.Time, f aggregate.Frequency, deltaValue int, deltaVectorValue []int64) {
	upsertAggregate.Call(ctx, namespace, name, typ, t, string(f), deltaValue, deltaVectorValue)
}
