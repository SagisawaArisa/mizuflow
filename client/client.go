package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/constraints"
	"mizuflow/pkg/logger"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"slices"

	"go.uber.org/zap"
)

type MizuClient struct {
	addr       string
	env        string
	namespaces []string
	apiKey     string
	httpClient *http.Client
	cacheFile  string

	snapshotIntervalMin time.Duration
	snapshotIntervalMax time.Duration

	mu       sync.RWMutex
	features map[string]v1.FeatureFlag
	lastRev  int64
	isDirty  bool

	ctx    context.Context
	cancel context.CancelFunc
}

type Option func(*MizuClient)

func WithCacheFile(path string) Option {
	return func(c *MizuClient) {
		c.cacheFile = path
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *MizuClient) {
		c.httpClient = client
	}
}

func WithSnapshotInterval(min, max time.Duration) Option {
	return func(c *MizuClient) {
		c.snapshotIntervalMin = min
		c.snapshotIntervalMax = max
	}
}

func NewMizuClient(addr, env, apiKey string, namespaces []string, opts ...Option) *MizuClient {
	ctx, cancel := context.WithCancel(context.Background())
	c := &MizuClient{
		addr:                addr,
		env:                 env,
		apiKey:              apiKey,
		namespaces:          namespaces,
		cacheFile:           ".mizu_cache.json",
		httpClient:          &http.Client{Timeout: 10 * time.Second},
		snapshotIntervalMin: 10 * time.Second,
		snapshotIntervalMax: 30 * time.Second,
		features:            make(map[string]v1.FeatureFlag),
		ctx:                 ctx,
		cancel:              cancel,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *MizuClient) Start() error {
	if err := c.fetchAll(); err != nil {
		logger.Warn("failed to fetch from server, attempting to load from local cache", zap.Error(err))
		if loadErr := c.loadSnapshot(); loadErr != nil {
			return fmt.Errorf("failed to fetch from server: %w, and failed to load from cache: %w", err, loadErr)
		}
		logger.Info("loaded features from local cache persistence")
	}
	go c.runWatchLoop()
	go c.runSnapshotLoop()
	return nil
}

func (c *MizuClient) fetchAll() error {
	nsParam := strings.Join(c.namespaces, ",")
	url := fmt.Sprintf("%s/v1/stream/snapshot?env=%s&namespace=%s", c.addr, c.env, nsParam)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Mizu-Key", c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("failed to fetch all features", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	var res struct {
		Data     []v1.FeatureFlag `json:"data"`
		Revision int64            `json:"revision"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		logger.Error("failed to decode features response", zap.Error(err))
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range res.Data {
		c.features[f.Key] = f
	}
	c.lastRev = res.Revision
	c.isDirty = true
	return nil
}

func (c *MizuClient) runWatchLoop() {
	backoff := time.Second
	maxBackoff := 30 * time.Second
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.mu.RLock()
			nsParam := strings.Join(c.namespaces, ",")
			url := fmt.Sprintf("%s/v1/stream/watch?last_rev=%d&env=%s&namespace=%s", c.addr, c.lastRev, c.env, nsParam)
			c.mu.RUnlock()

			// Use sub-context for request cancellation
			reqCtx, reqCancel := context.WithCancel(c.ctx)
			req, _ := http.NewRequestWithContext(reqCtx, "GET", url, nil)
			req.Header.Set("X-Mizu-Key", c.apiKey)
			resp, err := c.httpClient.Do(req)
			if err != nil {
				reqCancel()
				jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
				logger.Warn("SSE disconnected", zap.Error(err))
				time.Sleep(backoff + jitter)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			// Watchdog for heartbeats
			var lastActivity int64 = time.Now().Unix()
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-reqCtx.Done():
						return
					case <-ticker.C:
						if time.Now().Unix()-atomic.LoadInt64(&lastActivity) > 25 {
							logger.Warn("sse heartbeat timeout, reconnecting")
							reqCancel()
							return
						}
					}
				}
			}()

			backoff = time.Second
			scanner := bufio.NewScanner(resp.Body)

			var eventType string
			var dataBuffer bytes.Buffer

			for scanner.Scan() {
				atomic.StoreInt64(&lastActivity, time.Now().Unix())
				line := scanner.Text()
				if line == "" {
					// Process the accumulated message
					if eventType == "reset" {
						logger.Warn("received reset event, re-fetching all features")
						if err := c.fetchAll(); err != nil {
							logger.Error("failed to refetch features after reset", zap.Error(err))
						}
						// Close current stream
						reqCancel()
						break
					} else if eventType == "ping" {
						eventType = ""
						dataBuffer.Reset()
						continue
					} else if dataBuffer.Len() > 0 {
						var msg v1.Message
						if err := json.Unmarshal(dataBuffer.Bytes(), &msg); err == nil {
							c.handleUpdate(msg)
						} else {
							logger.Error("failed to unmarshal feature update", zap.Error(err))
						}
					}

					// Reset buffers for next message
					eventType = ""
					dataBuffer.Reset()
					continue
				}

				if strings.HasPrefix(line, "event: ") {
					eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				} else if strings.HasPrefix(line, "data:") {
					// Spec allows multiple data lines, joined by newline
					if dataBuffer.Len() > 0 {
						dataBuffer.WriteString("\n")
					}
					dataBuffer.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				}
			}
			reqCancel()
			resp.Body.Close()
		}
	}
}

func (c *MizuClient) handleUpdate(msg v1.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if msg.Revision <= c.lastRev {
		logger.Warn("stale revision received", zap.Int64("msg_rev", msg.Revision), zap.Int64("last_rev", c.lastRev))
		return
	}
	latency := time.Now().UnixMilli() - msg.UpdatedAt
	logger.Info("feature update received", zap.String("key", msg.Key), zap.String("action", string(msg.Action)), zap.Int64("rev", msg.Revision), zap.Int64("latency_ms", latency))
	switch msg.Action {
	case constraints.DELETE:
		delete(c.features, msg.Key)
		logger.Info("feature deleted", zap.String("key", msg.Key), zap.Int64("rev", msg.Revision))
	case constraints.PUT:
		c.features[msg.Key] = v1.FeatureFlag{
			Key:      msg.Key,
			Value:    msg.Value,
			Type:     msg.Type,
			Version:  msg.Version,
			Revision: msg.Revision,
		}
		logger.Info("feature updated", zap.String("key", msg.Key), zap.String("value", msg.Value), zap.Int64("rev", msg.Revision))
	default:
		logger.Warn("unknown action in feature update", zap.String("action", string(msg.Action)))
	}

	c.lastRev = msg.Revision
	c.isDirty = true
}

func (c *MizuClient) IsEnabled(key string, context map[string]string) bool {
	val, ok := c.evaluate(key, context)
	if !ok {
		return false
	}
	return val == "true" || val == "True" || val == "TRUE"
}

func (c *MizuClient) GetString(key string, defaultValue string, context map[string]string) string {
	val, ok := c.evaluate(key, context)
	if !ok {
		return defaultValue
	}
	return val
}

func (c *MizuClient) GetNumber(key string, defaultValue float64, context map[string]string) float64 {
	val, ok := c.evaluate(key, context)
	if !ok {
		return defaultValue
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

func (c *MizuClient) GetJSON(key string, target any, context map[string]string) error {
	val, ok := c.evaluate(key, context)
	if !ok {
		return fmt.Errorf("feature not found")
	}
	return json.Unmarshal([]byte(val), target)
}

func (c *MizuClient) evaluate(key string, context map[string]string) (string, bool) {
	c.mu.RLock()
	feature, ok := c.features[key]
	c.mu.RUnlock()

	if !ok {
		logger.Warn("key not found", zap.String("key", key))
		return "", false
	}

	if feature.Type != constraints.TypeStrategy {
		return feature.Value, true
	}
	var strategy v1.FeatureStrategy
	if err := json.Unmarshal([]byte(feature.Value), &strategy); err != nil {
		return feature.Value, true
	}

	for _, rule := range strategy.Rules {
		if c.matchRule(rule, context) {
			return rule.Result, true
		}
	}

	return strategy.DefaultValue, true
}

func (c *MizuClient) matchRule(rule v1.Rule, content map[string]string) bool {
	val, ok := content[rule.Attribute]
	if !ok {
		return false
	}

	switch rule.Operator {
	case "in":
		if slices.Contains(rule.Values, val) {
			return true
		}
	case "eq":
		return val == rule.Values[0]
	case "mod":
		// rule.Values[0] is expected to be an integer threshold between 0-100
		threshold, err := strconv.Atoi(rule.Values[0])
		if err != nil || threshold == 0 {
			return false
		}
		h := fnv.New32a()
		h.Write([]byte(val))
		hashVal := h.Sum32()
		return int(hashVal%100) < threshold
	}

	return false
}

type snapshot struct {
	Features map[string]v1.FeatureFlag `json:"features"`
	Revision int64                     `json:"revision"`
}

func (c *MizuClient) runSnapshotLoop() {
	var (
		minInterval = c.snapshotIntervalMin
		maxInterval = c.snapshotIntervalMax
	)
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-time.After(minInterval + time.Duration(rand.Int63n(int64(maxInterval-minInterval)))):
			c.mu.Lock()
			if !c.isDirty {
				c.mu.Unlock()
				continue
			}
			c.isDirty = false
			c.mu.Unlock()

			c.saveSnapshot()
		}
	}
}

func (c *MizuClient) saveSnapshot() {
	c.mu.RLock()
	data := snapshot{
		Features: c.features,
		Revision: c.lastRev,
	}
	bytes, err := json.Marshal(data)
	c.mu.RUnlock()

	if err != nil {
		logger.Error("failed to marshal snapshot", zap.Error(err))
		return
	}

	tmpFile := c.cacheFile + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0644); err != nil {
		logger.Error("failed to write temp snapshot file", zap.Error(err))
		return
	}
	if err := os.Rename(tmpFile, c.cacheFile); err != nil {
		logger.Error("failed to rename snapshot file", zap.Error(err))
	}
}

func (c *MizuClient) loadSnapshot() error {
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return err
	}

	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.features = s.Features
	c.lastRev = s.Revision
	return nil
}
