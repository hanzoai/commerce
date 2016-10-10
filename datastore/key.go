package datastore

import (
	"crowdstart.com/datastore/iface"
	"crowdstart.com/datastore/key"
)

type Key iface.Key

var EncodeKey = key.Encode
var DecodeKey = key.Decode
