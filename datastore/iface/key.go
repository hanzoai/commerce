package iface

import (
	aeds "google.golang.org/appengine/datastore"
)

// Interface for aeds.Key
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
