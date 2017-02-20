package counter

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"time"
)

const (
	defaultShards = 3
	configKind    = "_counterconfig"
	shardKind     = "_countershard"
)

type counterConfig struct {
	Shards int
}

type shard struct {
	// Shard name
	Name string `json:"name"`
	// Tag for filtering
	Tag string `json:"tag"`
	// Counter
	Count int `json:"count"`
	// Array
	Members []string `json:"members"`
	// Time
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func memcacheKey(name string) string {
	return shardKind + ":" + name
}

// Count retrieves the value of the named counter.
func CountByName(c appengine.Context, name string) (int, error) {
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

func CountByTag(c appengine.Context, tag string) (int, error) {
	total := 0
	mkey := memcacheKey(tag)
	if _, err := memcache.JSON.Get(c, mkey, &total); err == nil {
		return total, nil
	}
	q := datastore.NewQuery(shardKind).Filter("Tag =", tag)
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
