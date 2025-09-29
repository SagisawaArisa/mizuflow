package service

import "sync"

type RevisionBuffer struct {
	messages []Message
	size     int
	mu       sync.RWMutex
}

func NewRevisionBuffer(size int) *RevisionBuffer {
	return &RevisionBuffer{
		messages: make([]Message, 0, size),
		size:     size,
	}
}
func (b *RevisionBuffer) AddMessage(msg Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.messages) >= b.size {
		b.messages = b.messages[1:]
	}
	b.messages = append(b.messages, msg)
}

func (b *RevisionBuffer) GetSince(lastRev int64) ([]Message, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.messages) == 0 || b.messages[0].Revision > lastRev+1 {
		return nil, false
	}
	var result []Message
	for _, m := range b.messages {
		if m.Revision > lastRev {
			result = append(result, m)
		}
	}
	return result, true
}
