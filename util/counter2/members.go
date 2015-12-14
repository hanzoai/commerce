package counter

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

func Members(c appengine.Context, name string) ([]string, error) {
	members := make([]string, 0)
	mkey := memcacheKey(name)
	if _, err := memcache.JSON.Get(c, mkey, &members); err == nil {
		return members, nil
	}
	q := datastore.NewQuery(shardKind).Filter("Name =", name)
	for t := q.Run(c); ; {
		var s shard
		_, err := t.Next(&s)
		if err == datastore.Done {
			break
		}
		if err != nil {
			return members, err
		}
		members = append(members, s.Members...)
	}
	memcache.JSON.Set(c, &memcache.Item{
		Key:        mkey,
		Object:     &members,
		Expiration: 60,
	})
	return members, nil
}

func MemberExists(c appengine.Context, name, value string) bool {
	members, err := Members(c, name)
	if err != nil {
		return false
	}
	for _, member := range members {
		if member == value {
			return true
		}
	}

	return false
}

// Adds a member to the array if it does not exist
func AddSetMember(c appengine.Context, name, value string) error {
	if MemberExists(c, name, value) {
		return nil
	}
	return AddMember(c, name, value)
}

// Adds a member to the array on the shard
func AddMember(c appengine.Context, name, value string) error {
	AddMemberTask.Call(c, name, value)
	return nil
}
