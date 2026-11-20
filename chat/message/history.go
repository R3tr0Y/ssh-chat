package message

import (
	"fmt"
	"io"
	"slices"
	"sync"
)

const timestampFmt = "2006-01-02 15:04:05"

// History contains the history entries.
type History struct {
	sync.RWMutex
	entries []Message
	head    int
	size    int
	out     io.Writer
}

// NewHistory constructs a new history of the given size. Zero disables retention.
func NewHistory(size int) *History {
	return &History{
		entries: make([]Message, max(0, size)),
	}
}

// Add adds the given entry to the entries in the history.
func (h *History) Add(entry Message) {
	h.Lock()
	defer h.Unlock()

	if capacity := len(h.entries); capacity != 0 {
		h.entries[h.head] = entry
		h.head++
		if h.head == capacity {
			h.head = 0
		}
		h.size = min(h.size+1, capacity)
	}

	if h.out != nil {
		fmt.Fprintf(h.out, "[%s] %s\n", entry.Timestamp().UTC().Format(timestampFmt), entry.String())
	}
}

// Len returns the number of entries in the history.
func (h *History) Len() int {
	h.RLock()
	defer h.RUnlock()
	return h.size
}

// Get returns up to num of the most recent entries in oldest-first order.
func (h *History) Get(num int) []Message {
	h.RLock()
	defer h.RUnlock()

	num = min(max(0, num), h.size)
	entries := make([]Message, num)
	if num == 0 {
		return entries
	}

	start := h.head - num
	if start >= 0 {
		copy(entries, h.entries[start:h.head])
	} else {
		n := copy(entries, h.entries[len(h.entries)+start:])
		copy(entries[n:], h.entries[:h.head])
	}
	return entries
}

// Select returns up to num of the most recent matching entries in oldest-first order.
// The predicate runs under a read lock and must not call back into the history.
func (h *History) Select(num int, match func(Message) bool) []Message {
	h.RLock()
	defer h.RUnlock()

	entries := make([]Message, 0, min(max(0, num), h.size))
	i := h.head
	for remaining := h.size; remaining > 0 && len(entries) < num; remaining-- {
		if i == 0 {
			i = len(h.entries)
		}
		i--
		if entry := h.entries[i]; match(entry) {
			entries = append(entries, entry)
		}
	}
	slices.Reverse(entries)
	return entries
}

// SetOutput sets the output for logging added messages.
func (h *History) SetOutput(w io.Writer) {
	h.Lock()
	h.out = w
	h.Unlock()
}
