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
	s.Model.Init(db, s)
}

func (s *Submission) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Submission {
	s := new(Submission)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
