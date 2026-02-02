package service

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

type MockObserver struct{}

func (m *MockObserver) IncOnline()                          {}
func (m *MockObserver) DecOnline()                          {}
func (m *MockObserver) RecordPush()                         {}
func (m *MockObserver) ObservePushLatency(duration float64) {}
func (m *MockObserver) UpdateEventLag(lag int)              {}

func TestHub_Concurrency(t *testing.T) {
	hub := NewHub(&MockObserver{}, 100*time.Millisecond, 512)
	go hub.Run()

	var wg sync.WaitGroup
	// Parameters for race detection
	clientCount := 50
	msgCount := 200

	clients := make([]*Client, clientCount)

	// 1. Concurrent Registration
	for i := 0; i < clientCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := &Client{
				Send:       make(chan v1.Message, 50),
				Namespaces: map[string]bool{"default": true},
				Env:        "dev",
			}
			clients[idx] = c
			hub.Register <- c
		}(i)
	}
	wg.Wait()

	broadcastDone := make(chan struct{})

	// 2. Concurrent Broadcast
	go func() {
		for i := 0; i < msgCount; i++ {
			hub.Broadcast <- v1.Message{
				Key:       "test-key",
				Namespace: "default",
				Env:       "dev",
				Revision:  int64(i),
			}
			// Small delay to allow interleaving with unregister
			if i%10 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		close(broadcastDone)
	}()

	// 3. Concurrent Unregister (churn)
	go func() {
		for i := 0; i < clientCount/2; i++ {
			time.Sleep(2 * time.Millisecond)
			hub.Unregister <- clients[i]
		}
	}()

	// 4. Reader Consuming Loop
	var readWg sync.WaitGroup
	for i := 0; i < clientCount; i++ {
		readWg.Add(1)
		go func(c *Client) {
			defer readWg.Done()
			timeout := time.After(3 * time.Second)
			for {
				select {
				case _, ok := <-c.Send:
					if !ok {
						return // Channel closed by Hub (disconnect/unregister)
					}
				case <-broadcastDone:
					// Drain remaining
					for {
						select {
						case _, ok := <-c.Send:
							if !ok {
								return
							}
						default:
							return
						}
					}
				case <-timeout:
					return
				}
			}
		}(clients[i])
	}

	readWg.Wait()
}
