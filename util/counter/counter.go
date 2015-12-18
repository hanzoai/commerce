package counter

import (
	"fmt"
	"math/rand"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/delay"
	"appengine/memcache"
	"appengine/taskqueue"

	"crowdstart.com/util/log"
)

type counterConfig struct {
	Shards int
}

type shard struct {
	Name string
	// Counter
	Count int
	// Array
	Set map[string]bool
}

const (
	defaultShards = 3
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
	set := make(map[string]bool)
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

		for member, _ := range s.Set {
			set[member] = true
		}
	}

	for member, _ := range set {
		members = append(members, member)
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
	AddMemberTask.Call(c, name, value)
	return nil
}

// Increment increments the named counter by 1
func Increment(c appengine.Context, name string) error {
	return IncrementBy(c, name, 1)
}

// Increment increments the named counter by amount
func IncrementBy(c appengine.Context, name string, amount int) error {
	IncrementByTask.Call(c, name, amount)
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

var IncrementByTask *delay.Function
var AddMemberTask *delay.Function

func init() {
	IncrementByTask = delay.Func("IncrementByTask", func(c appengine.Context, name string, amount int) {
		log.Warn("INCREMENT BY", c)
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
			log.Panic("IncrementByTask Error %v", err, c)
		}
		var s shard
		err = datastore.RunInTransaction(c, func(c appengine.Context) error {
			shardName := fmt.Sprintf("%s-shard%d", name, rand.Intn(cfg.Shards))
			key := datastore.NewKey(c, shardKind, shardName, 0, nil)
			err := datastore.Get(c, key, &s)
			// A missing entity and a present entity will both work.
			if err != nil && err != datastore.ErrNoSuchEntity {
				panic(err)
			}
			s.Name = name
			s.Count += amount
			_, err = datastore.Put(c, key, &s)
			return err
		}, nil)
		if err == datastore.ErrConcurrentTransaction {
			IncreaseShards(c, name, 1)
			t, err := IncrementByTask.Task(c, name, amount)
			if err != nil {
				log.Panic("IncrementByTask Error %v", err, c)
			}

			t.Delay = time.Duration(rand.Intn(30) * 1000000)
			_, err = taskqueue.Add(c, t, "")
			if err != nil {
				log.Panic("IncrementByTask Error %v", err, c)
			}
			return
		}
		if err != nil {
			log.Panic("IncrementByTask Error %v", err, c)
		}
		memcache.IncrementExisting(c, memcacheKey(name), int64(amount))
	})

	AddMemberTask = delay.Func("AddMember", func(c appengine.Context, name, value string) {
		log.Warn("ADD MEMBER", c)
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
			log.Panic("AddMemberTask Error %v", err, c)
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
			s.Set[value] = true
			_, err = datastore.Put(c, key, &s)
			return err
		}, nil)
		if err == datastore.ErrConcurrentTransaction {
			IncreaseShards(c, name, 1)
			t, err := AddMemberTask.Task(c, name, value)
			if err != nil {
				log.Panic("AddMemberTask Error %v", err, c)
			}

			t.Delay = time.Duration(rand.Intn(30) * 1000000)
			_, err = taskqueue.Add(c, t, "")
			if err != nil {
				log.Panic("AddMemberTask Error %v", err, c)
			}
			return
		}
		if err != nil {
			log.Panic("AddMemberTask Error %v", err, c)
		}

		mkey := memcacheKey(name)
		var members map[string]bool
		if _, err := memcache.JSON.Get(c, mkey, &members); err == nil {
			members[value] = true
			memcache.JSON.Set(c, &memcache.Item{
				Key:        mkey,
				Object:     &members,
				Expiration: 60,
			})
		}
	})
}
