package submission

import (
	"crowdstart.com/datastore"

	. "crowdstart.com/models"
)

var kind = "submission"

func (s Submission) Kind() string {
	return kind
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
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
