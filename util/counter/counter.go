package counter

import (
	"fmt"
	"math/rand"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

type counterConfig struct {
	Shards int
}

type shard struct {
	Name string
	// Counter
	Count int
	// Array
	Members []string
}

const (
	defaultShards = 20
	configKind    = "GeneralCounterShardConfig"
	shardKind     = "GeneralCounterShard"
)

func memcacheKey(name string) string {
	return shardKind + ":" + name
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

// Count retrieves the value of the named counter.
func Count(c appengine.Context, name string) (int, error) {
	total := 0
	mkey := memcacheKey(name)
	if _, err := memcache.JSON.Get(c, mkey, &total); err == nil {
		return total, nil
	}
	q := datastore.NewQuery(shardKind).Filter("Name =", name)
	for t := q.Run(c); ; {
		var s shard
		_, err := t.Next(&s)
		if err == datastore.Done {
			break
		}
		if err != nil {
			return total, err
		}
		total += s.Count
	}
	memcache.JSON.Set(c, &memcache.Item{
		Key:        mkey,
		Object:     &total,
		Expiration: 60,
	})
	return total, nil
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
	// Get counter config.
	var cfg counterConfig
	ckey := datastore.NewKey(c, configKind, name, 0, nil)
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		err := datastore.Get(c, ckey, &cfg)
		if err == datastore.ErrNoSuchEntity {
			cfg.Shards = defaultShards
			_, err = datastore.Put(c, ckey, &cfg)
		}
		return err
	}, nil)
	if err != nil {
		return err
	}
	var s shard
	err = datastore.RunInTransaction(c, func(c appengine.Context) error {
		shardName := fmt.Sprintf("%s-shard%d", name, rand.Intn(cfg.Shards))
		key := datastore.NewKey(c, shardKind, shardName, 0, nil)
		err := datastore.Get(c, key, &s)
		// A missing entity and a present entity will both work.
		if err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		s.Name = name
		s.Members = append(s.Members, value)
		_, err = datastore.Put(c, key, &s)
		return err
	}, nil)
	if err != nil {
		return err
	}

	mkey := memcacheKey(name)
	var members []string
	if _, err := memcache.JSON.Get(c, mkey, &members); err == nil {
		memcache.JSON.Set(c, &memcache.Item{
			Key:        mkey,
			Object:     &s.Members,
			Expiration: 60,
		})
	}
	return nil
}

// Increment increments the named counter by 1
func Increment(c appengine.Context, name string) error {
	return IncrementBy(c, name, 1)
}

// Increment increments the named counter by amount
func IncrementBy(c appengine.Context, name string, amount int) error {
	// Get counter config.
	var cfg counterConfig
	ckey := datastore.NewKey(c, configKind, name, 0, nil)
	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		err := datastore.Get(c, ckey, &cfg)
		if err == datastore.ErrNoSuchEntity {
			cfg.Shards = defaultShards
			_, err = datastore.Put(c, ckey, &cfg)
		}
		return err
	}, nil)
	if err != nil {
		return err
	}
	var s shard
	err = datastore.RunInTransaction(c, func(c appengine.Context) error {
		shardName := fmt.Sprintf("%s-shard%d", name, rand.Intn(cfg.Shards))
		key := datastore.NewKey(c, shardKind, shardName, 0, nil)
		err := datastore.Get(c, key, &s)
		// A missing entity and a present entity will both work.
		if err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		s.Name = name
		s.Count += amount
		_, err = datastore.Put(c, key, &s)
		return err
	}, nil)
	if err != nil {
		return err
	}
	memcache.IncrementExisting(c, memcacheKey(name), int64(amount))
	return nil
}

// IncreaseShards increases the number of shards for the named counter to n.
// It will never decrease the number of shards.
func IncreaseShards(c appengine.Context, name string, n int) error {
	ckey := datastore.NewKey(c, configKind, name, 0, nil)
	return datastore.RunInTransaction(c, func(c appengine.Context) error {
		var cfg counterConfig
		mod := false
		err := datastore.Get(c, ckey, &cfg)
		if err == datastore.ErrNoSuchEntity {
			cfg.Shards = defaultShards
			mod = true
		} else if err != nil {
			return err
		}
		if cfg.Shards < n {
			cfg.Shards = n
			mod = true
		}
		if mod {
			_, err = datastore.Put(c, ckey, &cfg)
		}
		return err
	}, nil)
}
