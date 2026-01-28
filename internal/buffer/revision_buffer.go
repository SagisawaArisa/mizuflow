package buffer

import (
	v1 "mizuflow/pkg/api/v1"
	"sort"
	"sync"
)

type RevisionBuffer struct {
	mu       sync.RWMutex
	messages []v1.Message
	size     int
	head     int
	isFull   bool
}

func NewRevisionBuffer(size int) *RevisionBuffer {
	if size <= 0 {
		size = 1000
	}
	return &RevisionBuffer{
		messages: make([]v1.Message, size),
		size:     size,
		head:     0,
		isFull:   false,
	}
}

func (b *RevisionBuffer) AddMessage(msg v1.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.messages[b.head] = msg
	b.head = (b.head + 1) % b.size
	if b.head == 0 {
		b.isFull = true
	}
}

func (b *RevisionBuffer) GetSince(lastRev int64) ([]v1.Message, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := b.head
	start := 0
	if b.isFull {
		count = b.size
		start = b.head
	}

	if count == 0 {
		return nil, true
	}

	oldestRev := b.messages[start].Revision

	// If lastRev is older than oldestRev, need full resync
	if lastRev < oldestRev {
		return nil, false
	}

	// Binary Search
	// Logical index range: [0, count) maps to Physical index: (start + i) % size
	searchFunc := func(i int) bool {
		physIdx := (start + i) % b.size
		return b.messages[physIdx].Revision > lastRev
	}

	idx := sort.Search(count, searchFunc)

	// If all messages are <= lastRev, return empty
	if idx == count {
		return nil, true
	}

	result := make([]v1.Message, 0, count-idx)
	for i := idx; i < count; i++ {
		physIdx := (start + i) % b.size
		result = append(result, b.messages[physIdx])
	}

	return result, true
}
