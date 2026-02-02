package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Configuration
var (
	targetURL  = flag.String("url", "http://localhost:8080/v1/stream/watch?env=dev&namespace=default", "Target SSE URL")
	sdkKey     = flag.String("key", "mizu-admin-key-1", "SDK Key")
	totalVUs   = flag.Int("c", 2000, "Total Virtual Users (Concurrency)")
	rampUp     = flag.Duration("ramp", 60*time.Second, "Ramp up duration")
	featureKey = flag.String("feature", "loadtest-latency-check", "Feature key to measure")
)

// Metrics
var (
	activeClients int64
	totalconnects int64
	connectErrors int64
	messagesRx    int64
	latencySum    int64 // milliseconds
	latencyCount  int64
)

type EventMessage struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedAt int64  `json:"updated_at"`
}

func main() {
	flag.Parse()

	fmt.Printf("🚀 Starting Load Test\n")
	fmt.Printf("   Target: %s\n", *targetURL)
	fmt.Printf("   VUs: %d\n", *totalVUs)
	fmt.Printf("   Ramp: %v\n", *rampUp)

	// Disable HTTP/2 for simpler SSE handling in this load test if needed,
	// but standard client usually negotiates fine.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport.(*http.Transport).MaxIdleConns = *totalVUs
	http.DefaultTransport.(*http.Transport).MaxConnsPerHost = *totalVUs

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Metric Reporter
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentActive := atomic.LoadInt64(&activeClients)
				total := atomic.LoadInt64(&totalconnects)
				errs := atomic.LoadInt64(&connectErrors)
				msgs := atomic.SwapInt64(&messagesRx, 0)
				latSum := atomic.SwapInt64(&latencySum, 0)
				latCnt := atomic.SwapInt64(&latencyCount, 0)

				avgLat := float64(0)
				if latCnt > 0 {
					avgLat = float64(latSum) / float64(latCnt)
				}

				fmt.Printf("[%s] Active: %d | Total: %d | Errors: %d | Msgs/s: %d | Avg Latency: %.2f ms\n",
					time.Now().Format("15:04:05"), currentActive, total, errs, msgs, avgLat)
			}
		}
	}()

	// Ramp-up Logic
	interval := *rampUp / time.Duration(*totalVUs)
	for i := 0; i < *totalVUs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runClient(ctx, id)
		}(i)
		time.Sleep(interval)
	}

	// Keep alive
	fmt.Println("✅ All VUs launched. Waiting...")
	wg.Wait()
}

func runClient(ctx context.Context, id int) {
	req, err := http.NewRequestWithContext(ctx, "GET", *targetURL, nil)
	if err != nil {
		fmt.Printf("Client %d error: %v\n", id, err)
		return
	}

	req.Header.Set("X-Mizu-Key", *sdkKey)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 0, // Infinite timeout for SSE
	}

	resp, err := client.Do(req)
	if err != nil {
		if atomic.AddInt64(&connectErrors, 1) == 1 {
			fmt.Printf("Error connecting: %v\n", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if atomic.AddInt64(&connectErrors, 1) == 1 {
			fmt.Printf("Error status code: %d\n", resp.StatusCode)
		}
		return
	}

	atomic.AddInt64(&activeClients, 1)
	atomic.AddInt64(&totalconnects, 1)
	defer atomic.AddInt64(&activeClients, -1)

	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// server closed or network error
			return
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			var msg EventMessage
			if err := json.Unmarshal([]byte(data), &msg); err == nil {
				atomic.AddInt64(&messagesRx, 1)

				// Calculate Latency if it matches our measurement key
				if msg.Key == *featureKey {
					// Value is expected to be a unix timestamp (ms) string
					sentTime, err := strconv.ParseInt(msg.Value, 10, 64)
					if err == nil {
						latency := time.Now().UnixMilli() - sentTime
						// Filter reasonable range to avoid clock skew weirdness
						if latency >= 0 && latency < 10000 {
							atomic.AddInt64(&latencySum, latency)
							atomic.AddInt64(&latencyCount, 1)
						}
					}
				}
			}
		}
	}
}
