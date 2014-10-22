package crowdstart

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
    // "appengine"
    // "appengine/datastore"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
    // "github.com/qedus/nds"
)

// TODO: Extract CRUD logic from add and remove functions and place in another
// function. DONE.

var cookieStore = sessions.NewCookieStore([]byte("thisissecret"))

type LineItem struct {
	id, quantity int
	price        float32
	name         string
}

type Cart struct {
	items        []LineItem
	last_updated int64
}

func init() {
	gob.Register(&LineItem{})
	gob.Register(&Cart{})

	router := mux.NewRouter().StrictSlash(false)
	n := negroni.New(
		negroni.NewLogger(),
		negroni.NewStatic(http.Dir("skully_fe")),
		negroni.HandlerFunc(checkSession),
	)

	router.Path("/add").Methods("POST").HandlerFunc(add)
	router.Path("/remove").Methods("POST").HandlerFunc(remove)

	n.UseHandler(router)
	http.Handle("/", n)
}

func checkSession(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	session := getSession(r)
	defer session.Save(r, w)
	_, success := getCart(session)

	if !success {
		session.Values["cart"] = Cart{
			items:        nil,
			last_updated: time.Now().Unix(),
		}
	}
	next(w, r)
}

func getSession(r *http.Request) *sessions.Session {
	session, err := cookieStore.Get(r, "crowdstart_cart")
	if err != nil {
		panic(err)
	}
	return session
}

func getCart(session *sessions.Session) (Cart, bool) {
	cart, success := session.Values["cart"].(Cart)
	return cart, success
}

func setCart(cart Cart, session *sessions.Session) {
	session.Values["cart"] = cart
}

// id, quantity, cart, session
type updateCart func(int, int, Cart) Cart

func modifier(w http.ResponseWriter, r *http.Request, f updateCart) {
	session := getSession(r)
	cart, _ := getCart(session)

	defer func() {
		cart.last_updated = time.Now().Unix()
		setCart(cart, session)
		session.Save(r, w)
		js, err := json.Marshal(cart)
		if err == nil {
			fmt.Fprint(w, js)
		} else {
			fmt.Fprintf(w, "JSON error: %s", err)
		}
	}()

	formError := false
	itemId := r.FormValue("itemId")
	id, err := strconv.Atoi(itemId)
	if err != nil {
		fmt.Fprintf(w, "Unable to parse itemId (%s)", itemId)
		formError = true
	}

	quantity := r.FormValue("quantity")
	qi, err := strconv.Atoi(quantity)

	if err != nil {
		fmt.Fprintf(w, "Unable to parse quantity (%s)", quantity)
		formError = true
	}

	if formError {
		fmt.Fprintln(w, "Invalid form")
		return
	}

	cart = f(id, qi, cart)
}

func add(w http.ResponseWriter, r *http.Request) {
	//cart [itemId quantity]
	//TODO check if itemId correlates with catalog
	modifier(w, r, func(id, qi int, cart Cart) Cart {
		processed := false
		for _, item := range cart.items {
			if item.id == id {
				item.quantity = qi
				break
			}
		}
		if !processed {
			cart.items = append(cart.items, LineItem{id: id, quantity: qi, price: 9000.01, name: "SKULLY AR-1"})
		}
		return cart
	})
}

func remove(w http.ResponseWriter, r *http.Request) {
	//cart [itemId]
	//TODO check if itemId correlates with catalog
	modifier(w, r, func(id, qi int, cart Cart) Cart {
		loc := -1
		for i, item := range cart.items {
			if item.id == id {
				loc = i
				break
			}
		}
		if loc > 0 {
			cart.items = append(cart.items[:loc], cart.items[loc+1:]...)
		}
		return cart
	})
}
