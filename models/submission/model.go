package submission

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (s Submission) Kind() string {
	return "submission"
}

func (s *Submission) Init(db *datastore.Datastore) {
	s.Model = mixin.Model{Db: db, Entity: s}
}

func (s *Submission) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Submission {
	return new(Submission).New(db).(*Submission)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
