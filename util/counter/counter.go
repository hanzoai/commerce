package counter

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	aeds "google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/taskqueue"

	"hanzo.io/datastore"
	"hanzo.io/delay"
	"hanzo.io/log"
	"hanzo.io/util/json"
)

type counterConfig struct {
	Shards int
}

type Period string

const (
	None    Period = "none"
	Total   Period = "total"
	Hourly  Period = "hourly"
	Daily   Period = "daily"
	Weekly  Period = "weekly"
	Monthly Period = "monthly"
	Yearly  Period = "yearly"
)

type Shard struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
	// Counter
	Count   int    `json:"count"`
	StoreId string `json:"storeId"`
	Geo     string `json:"geo"`
	// Array
	Set  map[string]bool `datastore:"-"`
	Set_ string          `datastore:",noindex" json:"set"`

	Period Period `json:"period"`

	Time time.Time `json:"time"`
}

func (s *Shard) Load(ps []aeds.Property) (err error) {
	// Load supported properties
	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Set_) > 0 {
		err = json.DecodeBytes([]byte(s.Set_), &s.Set)
	}

	return err
}

func (s *Shard) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	s.Set_ = string(json.EncodeBytes(&s.Set))

	// Save properties
	return datastore.SaveStruct(s)
}

const (
	DefaultShards = 3
	ConfigKind    = "_counterconfig"
	ShardKind     = "_countershard"
)

func memcacheKey(name string) string {
	return ShardKind + ":" + name
}

func MemberExists(c context.Context, name string, value string) bool {
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

func Members(c context.Context, name string) ([]string, error) {
	set := make(map[string]bool)
	members := make([]string, 0)
	mkey := memcacheKey(name)
	if _, err := memcache.JSON.Get(c, mkey, &members); err == nil {
		return members, nil
	}
	q := aeds.NewQuery(ShardKind).Filter("Name =", name)
	for t := q.Run(c); ; {
		var s Shard
		_, err := t.Next(&s)
		if err == aeds.Done {
			break
		}
		if err != nil {
			return members, err
		}

		if s.Set == nil {
			continue
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
func Count(c context.Context, name string) (int, error) {
	total := 0
	mkey := memcacheKey(name)
	if _, err := memcache.JSON.Get(c, mkey, &total); err == nil {
		return total, nil
	}
	q := aeds.NewQuery(ShardKind).Filter("Name =", name)
	for t := q.Run(c); ; {
		var s Shard
		_, err := t.Next(&s)
		if err == aeds.Done {
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

// func CountByTag(c context.Context, tag, storeId string, p Period, start, end time.Time) (int, error) {
// 	total := 0
// 	q := aeds.NewQuery(ShardKind).Filter("Tag =", tag).Filter("CreatedAt>=", start).Filter("CreatedAt<=", end)
// 	for t := q.Run(c); ; {
// 		var s Shard
// 		_, err := t.Next(&s)
// 		if err == aeds.Done {
// 			break
// 		}
// 		if err != nil {
// 			return total, err
// 		}
// 		total += s.Count
// 	}
// 	return total, nil
// }

// Adds a member to the array if it does not exist
func AddSetMember(c context.Context, name, tag, storeId, geo string, p Period, value string, t time.Time) error {
	if MemberExists(c, name, value) {
		return nil
	}
	return AddMember(c, name, tag, storeId, geo, p, value, t)
}

// Adds a member to the array on the Shard
func AddMember(c context.Context, name, tag, storeId, geo string, p Period, value string, t time.Time) error {
	AddMemberTask.Call(c, name, tag, storeId, geo, p, value, t)
	return nil
}

// Increment increments the named counter by 1
func Increment(c context.Context, name, tag, storeId, geo string, p Period, t time.Time) error {
	return IncrementBy(c, name, tag, storeId, geo, p, 1, t)
}

// Increment increments the named counter by amount
func IncrementBy(c context.Context, name, tag, storeId, geo string, p Period, amount int, t time.Time) error {
	IncrementByTask.Call(c, name, tag, storeId, geo, p, amount, t)
	return nil
}

// IncreaseShards increases the number of Shards for the named counter to n.
// It will never decrease the number of Shards.
func IncreaseShards(c context.Context, name string, n int) error {
	ckey := aeds.NewKey(c, ConfigKind, name, 0, nil)
	return datastore.RunInTransaction(c, func(db *datastore.Datastore) error {
		var cfg counterConfig
		mod := false
		err := aeds.Get(c, ckey, &cfg)
		if err == aeds.ErrNoSuchEntity {
			cfg.Shards = DefaultShards
			mod = true
		} else if err != nil {
			return err
		}
		if cfg.Shards < n {
			cfg.Shards = n
			mod = true
		}
		if mod {
			_, err = aeds.Put(c, ckey, &cfg)
		}
		return err
	}, nil)
}

var IncrementByTask *delay.Function
var AddMemberTask *delay.Function

func init() {
	IncrementByTask = delay.Func("IncrementByTask", func(c context.Context, name, tag, storeId, geo string, p Period, amount int, t time.Time) {
		log.Debug("INCREMENT %s BY %d", name, amount, c)
		// Get counter config.
		var cfg counterConfig
		ckey := aeds.NewKey(c, ConfigKind, name, 0, nil)
		err := datastore.RunInTransaction(c, func(db *datastore.Datastore) error {
			err := aeds.Get(c, ckey, &cfg)
			if err == aeds.ErrNoSuchEntity {
				cfg.Shards = DefaultShards
				_, err = aeds.Put(c, ckey, &cfg)
			}
			return err
		}, nil)
		err = datastore.IgnoreFieldMismatch(err)
		if err != nil {
			log.Panic("IncrementByTask Error %v", err, c)
		}
		var s Shard
		err = datastore.RunInTransaction(c, func(db *datastore.Datastore) error {
			ShardName := fmt.Sprintf("%s-Shard%d", name, rand.Intn(cfg.Shards))
			key := aeds.NewKey(c, ShardKind, ShardName, 0, nil)
			err := aeds.Get(c, key, &s)
			// A missing entity and a present entity will both work.
			err = datastore.IgnoreFieldMismatch(err)
			if err != nil && err != aeds.ErrNoSuchEntity {
				panic(err)
			}
			s.Name = name
			s.Tag = tag
			s.StoreId = storeId
			s.Period = p
			s.Geo = geo
			s.Count += amount
			s.Time = t
			_, err = aeds.Put(c, key, &s)
			return err
		}, nil)
		if err == aeds.ErrConcurrentTransaction {
			IncreaseShards(c, name, 1)
			t, err := IncrementByTask.Task(name, tag, storeId, geo, p, amount, t)
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
		err = datastore.IgnoreFieldMismatch(err)
		if err != nil {
			log.Panic("IncrementByTask Error %v", err, c)
		}
		memcache.IncrementExisting(c, memcacheKey(name), int64(amount))
	})

	AddMemberTask = delay.Func("AddMember", func(c context.Context, name, tag, storeId, geo string, p Period, value string, t time.Time) {
		log.Debug("ADD MEMBER", c)
		// Get counter config.
		var cfg counterConfig
		ckey := aeds.NewKey(c, ConfigKind, name, 0, nil)
		err := datastore.RunInTransaction(c, func(db *datastore.Datastore) error {
			err := aeds.Get(c, ckey, &cfg)
			if err == aeds.ErrNoSuchEntity {
				cfg.Shards = DefaultShards
				_, err = aeds.Put(c, ckey, &cfg)
			}
			return err
		}, nil)
		err = datastore.IgnoreFieldMismatch(err)
		if err != nil {
			log.Panic("AddMemberTask Error %v", err, c)
		}
		var s Shard
		err = datastore.RunInTransaction(c, func(db *datastore.Datastore) error {
			ShardName := fmt.Sprintf("%s-Shard%d", name, rand.Intn(cfg.Shards))
			key := aeds.NewKey(c, ShardKind, ShardName, 0, nil)
			err := aeds.Get(c, key, &s)
			// A missing entity and a present entity will both work.
			err = datastore.IgnoreFieldMismatch(err)
			if err != nil && err != aeds.ErrNoSuchEntity {
				return err
			}
			s.Name = name
			if s.Set == nil {
				s.Set = make(map[string]bool)
			}
			s.Set[value] = true
			s.StoreId = storeId
			s.Period = p
			s.Geo = geo
			s.Time = t
			_, err = aeds.Put(c, key, &s)
			return err
		}, nil)
		if err == aeds.ErrConcurrentTransaction {
			IncreaseShards(c, name, 1)
			t, err := AddMemberTask.Task(name, tag, storeId, geo, p, value)
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
		err = datastore.IgnoreFieldMismatch(err)
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
