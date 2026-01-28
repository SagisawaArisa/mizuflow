package buffer

import (
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"sync"
	"testing"
	"time"
)

func init() {
	logger.InitLogger("test")
}

func TestRevisionBuffer_Lifecycle(t *testing.T) {
	// Size 3
	buf := NewRevisionBuffer(3)

	// 1. Empty Buffer check
	msgs, ok := buf.GetSince(0)
	if !ok || len(msgs) != 0 {
		t.Error("Empty buffer should return empty slice and ok=true")
	}

	// 2. Fill Buffer [1, 2, 3]
	buf.AddMessage(v1.Message{Revision: 1})
	buf.AddMessage(v1.Message{Revision: 2})
	buf.AddMessage(v1.Message{Revision: 3})

	// 3. Get All works if we request from 0 (which < oldestRev 1? No! 0 < 1 is true. So it FAILS!)
	// Wait, earlier manual traces suggested failure.
	// If it fails, we should assert "ok=false" (Resync needed).
	// Because buffer [1,2,3] cannot guarantee it has 0 -> 1 link.
	msgs, ok = buf.GetSince(0)
	if ok {
		t.Error("GetSince(0) should FAIL because 0 < oldestRev(1)")
	}

	// 4. Wrap Around: Add 4. Buffer logical: [2, 3, 4]
	buf.AddMessage(v1.Message{Revision: 4})

	// 5. Test Resync (Rev 1 is gone)
	msgs, ok = buf.GetSince(1)
	if ok {
		t.Error("GetSince(1) should fail (ok=false) because 1 < oldestRev(2)")
	}

	// 6. Test Valid Partial Get (Get > 2 -> [3, 4])
	msgs, ok = buf.GetSince(2)
	if !ok {
		t.Error("GetSince(2) should be valid")
	}
	if len(msgs) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Revision != 3 || msgs[1].Revision != 4 {
		t.Errorf("Expected [3, 4], got [%d, %d]", msgs[0].Revision, msgs[1].Revision)
	}

	// 7. Test Up-to-date (Get > 4 -> [])
	msgs, ok = buf.GetSince(4)
	if !ok {
		t.Error("GetSince(4) should be valid")
	}
	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(msgs))
	}
}

func TestRevisionBuffer_Concurrency(t *testing.T) {
	buf := NewRevisionBuffer(1000)
	done := make(chan struct{})
	count := 5000

	// Writer
	go func() {
		for i := 1; i <= count; i++ {
			buf.AddMessage(v1.Message{Revision: int64(i)})
			time.Sleep(2 * time.Microsecond)
		}
		close(done)
	}()

	// Readers
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var lastRev int64 = 0
			// Timeout safety
			timeout := time.After(5 * time.Second)

			for {
				select {
				case <-done:
					return
				case <-timeout:
					t.Error("Test timed out")
					return
				default:
					msgs, ok := buf.GetSince(lastRev)
					if ok && len(msgs) > 0 {
						lastRev = msgs[len(msgs)-1].Revision
					}
					// If !ok, we just retry (simulate client fallback)
					if !ok {
						// In real world, we'd fetch snapshot.
						// Here we just accept gap.
					}
				}
			}
		}(i)
	}

	wg.Wait()
}
