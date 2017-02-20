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

	"hanzo.io/util/log"
)

var IncrementByTask *delay.Function
var AddMemberTask *delay.Function

func incrementByTask(c appengine.Context, name, tag string, amount int) {
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
		if err == datastore.ErrNoSuchEntity {
			s.CreatedAt = time.Now()
		} else if err != nil {
			return err
		}
		s.Name = name
		s.Tag = tag
		s.Count += amount
		s.CreatedAt = time.Now()
		s.UpdatedAt = s.CreatedAt
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
}

func addMemberTask(c appengine.Context, name, tag, value string) {
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
		if err == datastore.ErrNoSuchEntity {
			s.CreatedAt = time.Now()
		} else if err != nil {
			return err
		}
		s.Name = name
		s.Tag = tag
		s.Members = append(s.Members, value)
		s.UpdatedAt = s.CreatedAt
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
	var members []string
	if _, err := memcache.JSON.Get(c, mkey, &members); err == nil {
		memcache.JSON.Set(c, &memcache.Item{
			Key:        mkey,
			Object:     &s.Members,
			Expiration: 60,
		})
	}
}

func init() {
	AddMemberTask = delay.Func("AddMember", addMemberTask)
	IncrementByTask = delay.Func("IncrementByTask", incrementByTask)
}
