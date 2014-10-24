package store

import (
	"appengine"
	"appengine/datastore"
	"github.com/gin-gonic/gin"
	//	"github.com/twinj/uuid"
	"bytes"
	"encoding/gob"
	"log"
	"net/http"
	"time"
)

type LineItem struct {
	Id, quantity int
	Price        float32
	Name         string
}

type Cart struct {
	Id                    string
	Items                 []LineItem
	Created, Last_updated int64
}

func (cart *Cart) encode() ([]byte, error) {
	//http://stackoverflow.com/a/12854659
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(*cart)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func decode(buf []byte) (Cart, error) {
	var cart Cart
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&cart)
	return cart, err
}

func SetCart(c appengine.Context, cart Cart) (string, error) {
 	if cartEnc, err := cart.encode(); err == nil {
		var key *datastore.Key
		if cart.Id == "" {
			key = datastore.NewIncompleteKey(c, "cart", nil)
		} else {
			key = datastore.NewKey(c, "cart", cart.Id, 0, nil)
		}
		key, err := datastore.Put(c, key, &cartEnc)
		if err == nil {
			return key.StringID(), nil
		}
		return "", nil
	} else {
		return "", err
	}
}

func GetCart(c appengine.Context, id string) (Cart, error) {
	var buf []byte
	key := datastore.NewKey(c, "cart", id, 0, nil)
	err := datastore.Get(c, key, &buf)
	if err == nil {
		return decode(buf)
	} else {
		return Cart{}, err
	}
}
