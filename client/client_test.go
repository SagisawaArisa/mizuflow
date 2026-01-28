package client

import (
	"fmt"
	"hash/fnv"
	"math"
	v1 "mizuflow/pkg/api/v1"
	"mizuflow/pkg/logger"
	"strconv"
	"testing"
)

func init() {
	logger.InitLogger("test")
}

func TestMatchRule(t *testing.T) {
	c := &MizuClient{}

	isModHit := func(val string, threshold int) bool {
		h := fnv.New32a()
		h.Write([]byte(val))
		return int(h.Sum32()%100) < threshold
	}

	tests := []struct {
		name     string
		rule     v1.Rule
		context  map[string]string
		expected bool
	}{
		{
			name:     "Operator IN - Match",
			rule:     v1.Rule{Attribute: "role", Operator: "in", Values: []string{"admin", "editor"}},
			context:  map[string]string{"role": "editor"},
			expected: true,
		},
		{
			name:     "Operator IN - No Match",
			rule:     v1.Rule{Attribute: "role", Operator: "in", Values: []string{"admin", "editor"}},
			context:  map[string]string{"role": "viewer"},
			expected: false,
		},
		{
			name:     "Operator IN - Missing Attribute",
			rule:     v1.Rule{Attribute: "role", Operator: "in", Values: []string{"admin"}},
			context:  map[string]string{"group": "eng"},
			expected: false,
		},
		{
			name:     "Operator EQ - Match",
			rule:     v1.Rule{Attribute: "region", Operator: "eq", Values: []string{"us-east-1"}},
			context:  map[string]string{"region": "us-east-1"},
			expected: true,
		},
		{
			name:     "Operator EQ - No Match",
			rule:     v1.Rule{Attribute: "region", Operator: "eq", Values: []string{"us-east-1"}},
			context:  map[string]string{"region": "eu-west-1"},
			expected: false,
		},
		{
			name:     "Operator MOD - 50% Threshold - Hit",
			rule:     v1.Rule{Attribute: "userId", Operator: "mod", Values: []string{"50"}},
			context:  map[string]string{"userId": "user123"},
			expected: isModHit("user123", 50),
		},
		{
			name:     "Operator MOD - Invalid Threshold",
			rule:     v1.Rule{Attribute: "userId", Operator: "mod", Values: []string{"invalid"}},
			context:  map[string]string{"userId": "user123"},
			expected: false,
		},
		{
			name:     "Operator UNKNOWN - Should fail safely",
			rule:     v1.Rule{Attribute: "role", Operator: "unknown", Values: []string{"something"}},
			context:  map[string]string{"role": "something"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.matchRule(tt.rule, tt.context)
			if result != tt.expected {
				t.Errorf("matchRule() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestModDistribution(t *testing.T) {
	c := &MizuClient{}
	sampleSize := 10000
	thresholds := []int{10, 30, 50, 80}

	for _, threshold := range thresholds {
		t.Run(fmt.Sprintf("Threshold %d%%", threshold), func(t *testing.T) {
			matches := 0
			rule := v1.Rule{
				Attribute: "userId",
				Operator:  "mod",
				Values:    []string{strconv.Itoa(threshold)},
			}

			for i := 0; i < sampleSize; i++ {
				ctx := map[string]string{"userId": fmt.Sprintf("user-%d", i)}
				if c.matchRule(rule, ctx) {
					matches++
				}
			}

			percentage := float64(matches) / float64(sampleSize) * 100
			t.Logf("Distribution for %d%% threshold: %.2f%%", threshold, percentage)

			if math.Abs(percentage-float64(threshold)) > 2.5 {
				t.Errorf("Hash distribution poor: got %.2f%%, want ~%d%% (+/- 2.5%%)", percentage, threshold)
			}
		})
	}
}
