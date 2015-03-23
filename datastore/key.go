package datastore

import aeds "appengine/datastore"

// Alias datastore.Key with our Key interface
type Key interface {
	AppID() string
	Encode() string
	Equal(o *aeds.Key) bool
	GobDecode(buf []byte) error
	GobEncode() ([]byte, error)
	Incomplete() bool
	IntID() int64
	Kind() string
	MarshalJSON() ([]byte, error)
	Namespace() string
	Parent() *aeds.Key
	String() string
	StringID() string
	UnmarshalJSON(buf []byte) error
}
