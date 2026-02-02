package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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

	// Trigger/Sender Configuration
	enableTrigger  = flag.Bool("trigger", false, "Enable internal automated update trigger")
	adminBaseURL   = flag.String("admin", "http://localhost:8080", "Admin API Base URL")
	updateInterval = flag.Duration("interval", 5*time.Second, "Interval between updates")
	adminUser      = flag.String("user", "admin", "Admin Username")
	adminPass      = flag.String("pass", "admin123", "Admin Password")
)

// Metrics
var (
	activeClients   int64
	totalconnects   int64
	connectErrors   int64
	messagesRx      int64 // Delta (reset every tick)
	totalMessagesRx int64 // Cumulative
	latencySum      int64 // milliseconds
	latencyCount    int64
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
	// Important: Buffer connection limit to allow Trigger/Login + VUs
	bufferConns := 10
	http.DefaultTransport.(*http.Transport).MaxIdleConns = *totalVUs + bufferConns
	http.DefaultTransport.(*http.Transport).MaxConnsPerHost = *totalVUs + bufferConns

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pre-flight Trigger Setup (Run BEFORE launching massive connections)
	var triggerToken string
	if *enableTrigger {
		fmt.Println("🔫 Trigger enabled. Performing synchronous login check...")
		var err error
		triggerToken, err = login(*adminBaseURL, *adminUser, *adminPass)
		if err != nil {
			fmt.Printf("❌ TRIGGER LOGIN FAILED: %v\n", err)
			fmt.Println("⚠️  Continuing without auto-trigger...")
			*enableTrigger = false
		} else {
			fmt.Println("✅ Trigger logged in successfully. Token acquired.")
		}
	}

	// Metric Reporter
	go func() {
		// Log file
		f, err := os.Create("latency.csv")
		if err == nil {
			defer f.Close()
			f.WriteString("time,active_users,rate,avg_latency_ms\n")
		}

		// Refresh faster (500ms) for more "real-time" feel
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		var lastValidLatency float64
		startTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentActive := atomic.LoadInt64(&activeClients)
				// total := atomic.LoadInt64(&totalconnects) // Less important once stable
				// errs := atomic.LoadInt64(&connectErrors)

				// Get delta and reset
				msgsDelta := atomic.SwapInt64(&messagesRx, 0)

				// Get cumulative
				msgsTotal := atomic.LoadInt64(&totalMessagesRx)

				latSum := atomic.SwapInt64(&latencySum, 0)
				latCnt := atomic.SwapInt64(&latencyCount, 0)

				// Calculate Rate per second (since ticker is 500ms, rate is delta * 2)
				rate := msgsDelta * 2

				// Convert Microseconds sum to Milliseconds average
				avgLatMs := lastValidLatency
				if latCnt > 0 {
					avgLatMs = (float64(latSum) / float64(latCnt)) / 1000.0
					lastValidLatency = avgLatMs
				}

				// Write to CSV
				if f != nil {
					elapsed := time.Since(startTime).Seconds()
					fmt.Fprintf(f, "%.2f,%d,%d,%.3f\n", elapsed, currentActive, rate, avgLatMs)
				}

				// Clear screen line or just append? Append is safer for history.
				// Format: Time | Active | Rate | Total | Latency
				if currentActive > 0 || msgsTotal > 0 {
					fmt.Printf("\r[%s] Conns: %-5d | Rate: %-5d/s | Total Rx: %-6d | Latency: %.3f ms   ",
						time.Now().Format("15:04:05"), currentActive, rate, msgsTotal, avgLatMs)
				}
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
	fmt.Println("✅ All VUs launched.")

	if *enableTrigger && triggerToken != "" {
		fmt.Println("🔫 Starting background update loop...")
		go runTriggerLoop(triggerToken)
	} else {
		fmt.Println("ℹ️ Trigger disabled/failed. Waiting for external updates...")
	}

	fmt.Println("⏳ Waiting...")
	wg.Wait()
}

// --- Trigger Logic (Sender) ---

func runTriggerLoop(token string) {
	// Use a clean Transport for Trigger to avoid contention with SSE pool limits
	triggerTransport := &http.Transport{
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
		DisableKeepAlives: true, // Force new connection for each trigger (safer for debug)
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: triggerTransport,
	}

	ticker := time.NewTicker(*updateInterval)

	// Immediate first send
	sendUpdate(client, *adminBaseURL, token)

	for range ticker.C {
		sendUpdate(client, *adminBaseURL, token)
	}
}

func login(baseURL, user, pass string) (string, error) {
	loginURL := fmt.Sprintf("%s/v1/auth/login", baseURL)

	body := map[string]string{"username": user, "password": pass}
	jsonBody, _ := json.Marshal(body)

	// Use independent Transport to avoid blocking by SSE pool
	loginTransport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: loginTransport,
	}

	resp, err := client.Post(loginURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res["access_token"].(string), nil
}

func sendUpdate(client *http.Client, baseURL, token string) {
	// Debug: Print URL to confirm where we are sending
	updateURL := fmt.Sprintf("%s/v1/feature", baseURL)

	// Use String type for high precision timestamp
	// Using Microseconds to avoid 0ms latency on localhost
	ts := time.Now().UnixMicro()
	// Dynamic key to force new entries/logs
	dynamicKey := fmt.Sprintf("%s-%d", *featureKey, ts)

	payload := map[string]string{
		"namespace": "default",
		"env":       "dev",
		"key":       dynamicKey,
		"value":     fmt.Sprintf("%d", ts),
		"type":      "string",
	}
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", updateURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("⚠️ Trigger send failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("\n⚠️ Trigger FAILED status %d: %s\n", resp.StatusCode, string(b))
	}
}

// --- Client Logic (Receiver) ---

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
				atomic.AddInt64(&totalMessagesRx, 1)

				// Calculate Latency
				if strings.Contains(msg.Key, *featureKey) {
					tsStr := msg.Value
					// Try parsing microseconds
					sentTime, err := strconv.ParseInt(tsStr, 10, 64)
					if err != nil {
						// Fallback/Debug: Maybe it got quoted/escaped weirdly?
						// fmt.Printf("Parse Error: %s\n", tsStr)
						return
					}

					latencyMicros := time.Now().UnixMicro() - sentTime
					if latencyMicros >= 0 && latencyMicros < 10000000 {
						atomic.AddInt64(&latencySum, latencyMicros)
						atomic.AddInt64(&latencyCount, 1)
					}
				}
			}
		}
	}
}
