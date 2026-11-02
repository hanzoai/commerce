package db

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// row is a model with every shape the two codecs part over: a field tagged other
// than its name, a zero left out by omitempty, a storage field json leaves out, a
// runtime field the codec leaves out, a nested struct whose fields are tagged
// other than their names, a list of them, a large integer and a time.
type row struct {
	Code_     string            `json:"code"`
	IdemKey   string            `json:"key"`
	Credited  bool              `json:"credited,omitempty"`
	Count     int               `json:"count,omitempty"`
	Raw_      string            `json:"-"`
	Lines     []line            `json:"lines" datastore:"-"`
	Limits    *limits           `json:"limits,omitempty"`
	Seats     []limits          `json:"seats,omitempty"`
	Big       int64             `json:"big"`
	At        time.Time         `json:"at"`
	Refunds   map[string]int64  `json:"refunds,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type line struct {
	Desc string `json:"description"`
	N    int64  `json:"quantity"`
}

type limits struct {
	MinSeats  *int `json:"minSeats,omitempty"`
	MaxSeats_ int  `json:"most"`
}

func sample() row {
	n := 20
	at := time.Date(2026, 11, 2, 18, 0, 0, 123456789, time.UTC)
	return row{
		Code_: "50OFF", IdemKey: "k-1", Credited: false, Count: 0, Raw_: `[{"d":"x"}]`,
		Lines:  []line{{Desc: "seats", N: 20}},
		Limits: &limits{MinSeats: &n, MaxSeats_: 2000}, Seats: []limits{{MaxSeats_: 3}},
		Big: 9007199254740993, At: at, Refunds: map[string]int64{"refund:a": 9007199254740993},
		Labels: map[string]string{"tab": "enterprise"}, UpdatedAt: at,
	}
}

// What this codec writes on Postgres is read by it, by name, as SQLite's row is:
// every field this codec stores comes back, numbers digit for digit.
func TestARowWrittenNowReadsNow(t *testing.T) {
	in := sample()
	b, err := encodeRow(&in)
	if err != nil {
		t.Fatal(err)
	}
	var out row
	if err := unmarshalForDB(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code_ != in.Code_ || out.IdemKey != in.IdemKey || out.Raw_ != in.Raw_ || out.Big != in.Big ||
		!out.At.Equal(in.At) || out.Refunds["refund:a"] != in.Refunds["refund:a"] || out.Limits == nil ||
		*out.Limits.MinSeats != 20 || out.Limits.MaxSeats_ != 2000 || len(out.Seats) != 1 || out.Seats[0].MaxSeats_ != 3 ||
		len(out.Lines) != 1 || out.Lines[0].N != 20 {
		t.Fatalf("read back %+v", out)
	}
	// And it is stored under the names a filter states: Code_ as code_, IdemKey
	// as idemKey, the zero as a zero.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"code_", "idemKey", "credited", "count", "raw_"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("the row does not hold %q: %s", k, b)
		}
	}
}

// A Postgres row written before this codec — by encoding/json, under json tags,
// zeros left out — reads now: a field tagged other than its name is read by its
// tag, nested and listed ones too, and a runtime field from its tag.
func TestARowWrittenBeforeReadsNow(t *testing.T) {
	in := sample()
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatal(err)
	}
	var out row
	if err := unmarshalForDB(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Code_ != "50OFF" || out.IdemKey != "k-1" || out.Big != in.Big || out.Refunds["refund:a"] != in.Refunds["refund:a"] ||
		out.Limits == nil || out.Limits.MaxSeats_ != 2000 || len(out.Seats) != 1 || out.Seats[0].MaxSeats_ != 3 ||
		len(out.Lines) != 1 || out.Lines[0].Desc != "seats" || !out.UpdatedAt.Equal(in.UpdatedAt) {
		t.Fatalf("an old row read back %+v", out)
	}
}

// A row written now reads in an image from before this codec, which reads a
// Postgres row with encoding/json by json tags: rolling back loses nothing. The
// one thing it never held, a storage field json leaves out (Raw_), it still lacks.
func TestARowWrittenNowReadsBefore(t *testing.T) {
	in := sample()
	b, err := encodeRow(&in)
	if err != nil {
		t.Fatal(err)
	}
	var out row
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("the old reader refused the row: %v\n%s", err, b)
	}
	want := in
	want.Raw_ = ""
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("the old reader read %+v\nwant %+v\nfrom %s", out, want, b)
	}
}

// The same holds for every row of every registered model shape a value-level
// round trip can show: encodeRow then encoding/json gives back what
// encoding/json alone gives back.
func TestTheOldReaderReadsWhatItWroteFromTheNewRow(t *testing.T) {
	in := sample()
	old, _ := json.Marshal(&in)
	now, _ := encodeRow(&in)
	var a, b row
	if err := json.Unmarshal(old, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(now, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("the old reader reads the new row as %+v, its own as %+v", b, a)
	}
}
