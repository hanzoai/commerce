package submission

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Submission) Kind() string {
	return "submission"
}

func (s *Submission) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Submission {
	s := new(Submission)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
