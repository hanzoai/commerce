package watchlist

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/movie"

	. "github.com/hanzoai/commerce/types"
)

var kind = "watchlist"

func (w Watchlist) Kind() string {
	return kind
}

func (w *Watchlist) Init(db *datastore.Datastore) {
	w.Model.Init(db, w)
}

func (w *Watchlist) Defaults() {
	w.Movies = make([]movie.Movie, 0)
	w.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Watchlist {
	w := new(Watchlist)
	w.Init(db)
	w.Defaults()
	return w
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
