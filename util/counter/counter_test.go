package counter

import (
	"testing"
	"time"
)

// A shard's Set lives outside the datastore schema (Set_ carries it as JSON);
// a Save -> Load round trip must come back whole.
func TestShardSaveLoadRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 30, 12, 30, 0, 0, time.UTC)
	in := &Shard{
		Name:    "order.count",
		Tag:     "order.count",
		Count:   42,
		StoreId: "store-1",
		Geo:     "IN",
		Set:     map[string]bool{"alice": true, "bob": true},
		Period:  Daily,
		Time:    when,
	}

	ps, err := in.Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	out := &Shard{}
	if err := out.Load(ps); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if out.Name != in.Name || out.Tag != in.Tag || out.StoreId != in.StoreId || out.Geo != in.Geo {
		t.Errorf("identity fields changed: %+v", out)
	}
	if out.Count != 42 {
		t.Errorf("Count = %d, want 42", out.Count)
	}
	if out.Period != Daily {
		t.Errorf("Period = %q, want %q", out.Period, Daily)
	}
	if !out.Time.Equal(when) {
		t.Errorf("Time = %v, want %v", out.Time, when)
	}
	if len(out.Set) != 2 || !out.Set["alice"] || !out.Set["bob"] {
		t.Errorf("Set = %v, want alice+bob", out.Set)
	}
}

// An empty Set serializes and loads back without an error or phantom members.
func TestShardEmptySetRoundTrip(t *testing.T) {
	ps, err := (&Shard{Name: "n"}).Save()
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	out := &Shard{}
	if err := out.Load(ps); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Set) != 0 {
		t.Errorf("Set = %v, want empty", out.Set)
	}
}

func TestMemcacheKey(t *testing.T) {
	if got := memcacheKey("order.count"); got != "_countershard:order.count" {
		t.Errorf("memcacheKey = %q", got)
	}
}

func TestShardRetryJitterBounds(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := shardRetryJitter()
		if d < 0 || d >= 30*time.Millisecond {
			t.Fatalf("jitter %v outside [0, 30ms)", d)
		}
	}
}
