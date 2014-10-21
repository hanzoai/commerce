package crowdstart

import (
    "fmt"
    "net/http"
    "appengine"
    "appengine/datastore"
    "github.com/qedus/nds"
)

type Entity struct {
    Value string
}

func init() {
    http.HandleFunc("/", handler)
}

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

    k := datastore.NewKey(c, "Entity", "stringID", 0, nil)
    e := new(Entity)
    if err := nds.Get(c, k, e); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    old := e.Value
    e.Value = r.URL.Path

    if _, err := nds.Put(c, k, e); err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    fmt.Fprintf(w, "old=%q\nnew=%q\n", old, e.Value)
}
