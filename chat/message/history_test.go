package message

import (
	"strings"
	"testing"
)

func msgEqual(a []Message, b []Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
}

func TestHistory(t *testing.T) {
	var r, expected []Message
	var size int

	h := NewHistory(5)

	r = h.Get(10)
	expected = []Message{}
	if !msgEqual(r, expected) {
		t.Errorf("Got: %v, Expected: %v", r, expected)
	}

	h.Add(NewMsg("1"))

	if size = h.Len(); size != 1 {
		t.Errorf("Wrong size: %v", size)
	}

	r = h.Get(1)
	expected = []Message{NewMsg("1")}
	if !msgEqual(r, expected) {
		t.Errorf("Got: %v, Expected: %v", r, expected)
	}

	h.Add(NewMsg("2"))
	h.Add(NewMsg("3"))
	h.Add(NewMsg("4"))
	h.Add(NewMsg("5"))
	h.Add(NewMsg("6"))

	if size = h.Len(); size != 5 {
		t.Errorf("Wrong size: %v", size)
	}

	r = h.Get(2)
	expected = []Message{NewMsg("5"), NewMsg("6")}
	if !msgEqual(r, expected) {
		t.Errorf("Got: %v, Expected: %v", r, expected)
	}

	r = h.Get(10)
	expected = []Message{NewMsg("2"), NewMsg("3"), NewMsg("4"), NewMsg("5"), NewMsg("6")}
	if !msgEqual(r, expected) {
		t.Errorf("Got: %v, Expected: %v", r, expected)
	}
}

func BenchmarkHistoryGet(b *testing.B) {
	h := NewHistory(128)
	entry := NewMsg("hello")
	for range 129 {
		h.Add(entry)
	}
	b.ReportAllocs()
	for b.Loop() {
		h.Get(100)
	}
}

func TestDisabledHistoryStillLogs(t *testing.T) {
	h := NewHistory(0)
	var log strings.Builder
	h.SetOutput(&log)
	h.Add(NewMsg("logged"))
	if h.Len() != 0 || len(h.Get(10)) != 0 || len(h.Select(10, func(Message) bool { return true })) != 0 {
		t.Fatal("disabled history retained an entry")
	}
	if !strings.HasSuffix(log.String(), "] logged\n") {
		t.Fatalf("missing log entry: %q", log.String())
	}
}

func TestSelectWrappedHistory(t *testing.T) {
	h := NewHistory(4)
	for _, body := range []string{"first match", "second match", "no", "third match", "last match"} {
		h.Add(NewMsg(body))
	}
	got := h.Select(2, func(m Message) bool { return strings.Contains(m.String(), "match") })
	if want := []Message{NewMsg("third match"), NewMsg("last match")}; !msgEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
