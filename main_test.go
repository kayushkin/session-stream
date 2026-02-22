package main

import (
	"encoding/json"
	"testing"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{42, "42"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{85178, "85.2k"},
		{100000, "100.0k"},
		{123456, "123.5k"},
	}

	for _, tt := range tests {
		result := formatNumber(tt.input)
		if result != tt.expected {
			t.Errorf("formatNumber(%d) = %s; expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestProcessLineExtractsUsage(t *testing.T) {
	// Sample JSONL with realistic usage data
	jsonl := `{"message":{"role":"assistant","content":"Test response","usage":{"input":3,"output":196,"cacheRead":9155,"cacheWrite":75824,"totalTokens":85178}}}`

	result := processLine(jsonl)

	if result.Usage == nil {
		t.Fatal("Expected usage data to be extracted, got nil")
	}

	if result.Usage.TotalTokens != 85178 {
		t.Errorf("Expected totalTokens=85178, got %d", result.Usage.TotalTokens)
	}

	if result.Usage.Output != 196 {
		t.Errorf("Expected output=196, got %d", result.Usage.Output)
	}

	if result.Usage.Input != 3 {
		t.Errorf("Expected input=3, got %d", result.Usage.Input)
	}

	if result.Usage.CacheRead != 9155 {
		t.Errorf("Expected cacheRead=9155, got %d", result.Usage.CacheRead)
	}

	if result.Usage.CacheWrite != 75824 {
		t.Errorf("Expected cacheWrite=75824, got %d", result.Usage.CacheWrite)
	}
}

func TestFormatTokenUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    *Usage
		expected string
	}{
		{
			name:     "nil usage",
			usage:    nil,
			expected: "",
		},
		{
			name: "zero tokens",
			usage: &Usage{
				Input:       0,
				Output:      0,
				TotalTokens: 0,
			},
			expected: "",
		},
		{
			name: "realistic usage with cache",
			usage: &Usage{
				Input:       3,
				Output:      196,
				CacheRead:   9155,
				CacheWrite:  75824,
				TotalTokens: 85178,
			},
			expected: " \033[2mctx: 85.2k | out: 196\033[0m",
		},
		{
			name: "small numbers",
			usage: &Usage{
				Input:       50,
				Output:      100,
				TotalTokens: 500,
			},
			expected: " \033[2mctx: 500 | out: 100\033[0m",
		},
		{
			name: "only output",
			usage: &Usage{
				Input:       0,
				Output:      42,
				TotalTokens: 0,
			},
			expected: " \033[2mctx: 0 | out: 42\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTokenUsage(tt.usage)
			if result != tt.expected {
				t.Errorf("formatTokenUsage() = %q; expected %q", result, tt.expected)
			}
		})
	}
}

func TestProcessLineHandlesMultipleRoles(t *testing.T) {
	tests := []struct {
		name     string
		jsonl    string
		hasUsage bool
	}{
		{
			name:     "user message",
			jsonl:    `{"message":{"role":"user","content":"Hello"}}`,
			hasUsage: false,
		},
		{
			name:     "assistant message with usage",
			jsonl:    `{"message":{"role":"assistant","content":"Hi","usage":{"totalTokens":1000,"output":50}}}`,
			hasUsage: true,
		},
		{
			name:     "tool message",
			jsonl:    `{"message":{"role":"tool","content":"Result"}}`,
			hasUsage: false,
		},
		{
			name:     "system message",
			jsonl:    `{"message":{"role":"system","content":"System info"}}`,
			hasUsage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processLine(tt.jsonl)
			if tt.hasUsage && result.Usage == nil {
				t.Error("Expected usage data, got nil")
			}
			if !tt.hasUsage && result.Usage != nil {
				t.Error("Expected no usage data, got non-nil")
			}
		})
	}
}

func TestLogEntryDeserialization(t *testing.T) {
	// Verify that the LogEntry struct correctly deserializes all usage fields
	jsonl := `{"message":{"role":"assistant","content":"test","usage":{"input":3,"output":196,"cacheRead":9155,"cacheWrite":75824,"totalTokens":85178}},"timestamp":1640000000}`

	var entry LogEntry
	err := json.Unmarshal([]byte(jsonl), &entry)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if entry.Message.Usage == nil {
		t.Fatal("Usage should not be nil")
	}

	u := entry.Message.Usage
	if u.Input != 3 {
		t.Errorf("Input = %d; expected 3", u.Input)
	}
	if u.Output != 196 {
		t.Errorf("Output = %d; expected 196", u.Output)
	}
	if u.CacheRead != 9155 {
		t.Errorf("CacheRead = %d; expected 9155", u.CacheRead)
	}
	if u.CacheWrite != 75824 {
		t.Errorf("CacheWrite = %d; expected 75824", u.CacheWrite)
	}
	if u.TotalTokens != 85178 {
		t.Errorf("TotalTokens = %d; expected 85178", u.TotalTokens)
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		name     string
		cost     float64
		expected string
	}{
		{
			name:     "zero cost",
			cost:     0.0,
			expected: "$0.00",
		},
		{
			name:     "sub-cent cost",
			cost:     0.0049,
			expected: "$0.00",
		},
		{
			name:     "sub-cent rounded",
			cost:     0.005,
			expected: "$0.01",
		},
		{
			name:     "normal cost",
			cost:     0.4834,
			expected: "$0.48",
		},
		{
			name:     "dollar amount",
			cost:     1.234,
			expected: "$1.23",
		},
		{
			name:     "larger amount",
			cost:     12.99,
			expected: "$12.99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCost(tt.cost)
			if result != tt.expected {
				t.Errorf("formatCost(%f) = %s; expected %s", tt.cost, result, tt.expected)
			}
		})
	}
}

func TestLogEntryWithCost(t *testing.T) {
	// Verify that cost data is correctly deserialized
	jsonl := `{"message":{"role":"assistant","content":"test","usage":{"input":3,"output":196,"cacheRead":9155,"cacheWrite":75824,"totalTokens":85178,"cost":{"input":0.000015,"output":0.0049,"cacheRead":0.0045775,"cacheWrite":0.4739,"total":0.4834}}}}`

	var entry LogEntry
	err := json.Unmarshal([]byte(jsonl), &entry)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if entry.Message.Usage == nil {
		t.Fatal("Usage should not be nil")
	}

	if entry.Message.Usage.Cost == nil {
		t.Fatal("Cost should not be nil")
	}

	c := entry.Message.Usage.Cost
	if c.Input != 0.000015 {
		t.Errorf("Cost.Input = %f; expected 0.000015", c.Input)
	}
	if c.Output != 0.0049 {
		t.Errorf("Cost.Output = %f; expected 0.0049", c.Output)
	}
	if c.CacheRead != 0.0045775 {
		t.Errorf("Cost.CacheRead = %f; expected 0.0045775", c.CacheRead)
	}
	if c.CacheWrite != 0.4739 {
		t.Errorf("Cost.CacheWrite = %f; expected 0.4739", c.CacheWrite)
	}
	if c.Total != 0.4834 {
		t.Errorf("Cost.Total = %f; expected 0.4834", c.Total)
	}
}

func TestFormatTokenUsageWithCost(t *testing.T) {
	tests := []struct {
		name     string
		usage    *Usage
		expected string
	}{
		{
			name:     "nil usage",
			usage:    nil,
			expected: "",
		},
		{
			name: "usage with cost",
			usage: &Usage{
				Input:       3,
				Output:      196,
				TotalTokens: 85178,
				Cost: &Cost{
					Total: 0.4834,
				},
			},
			expected: " \033[2mctx: 85.2k | out: 196 | $0.48\033[0m",
		},
		{
			name: "usage without cost",
			usage: &Usage{
				Input:       50,
				Output:      100,
				TotalTokens: 500,
			},
			expected: " \033[2mctx: 500 | out: 100\033[0m",
		},
		{
			name: "usage with zero cost",
			usage: &Usage{
				Input:       50,
				Output:      100,
				TotalTokens: 500,
				Cost: &Cost{
					Total: 0.0,
				},
			},
			expected: " \033[2mctx: 500 | out: 100\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTokenUsage(tt.usage)
			if result != tt.expected {
				t.Errorf("formatTokenUsage() = %q; expected %q", result, tt.expected)
			}
		})
	}
}

func TestProcessLineExtractsCost(t *testing.T) {
	// Sample JSONL with cost data
	jsonl := `{"message":{"role":"assistant","content":"Test response","usage":{"input":3,"output":196,"cacheRead":9155,"cacheWrite":75824,"totalTokens":85178,"cost":{"input":0.000015,"output":0.0049,"cacheRead":0.0045775,"cacheWrite":0.4739,"total":0.4834}}}}`

	result := processLine(jsonl)

	if result.Usage == nil {
		t.Fatal("Expected usage data to be extracted, got nil")
	}

	if result.Usage.Cost == nil {
		t.Fatal("Expected cost data to be extracted, got nil")
	}

	if result.Usage.Cost.Total != 0.4834 {
		t.Errorf("Expected cost.total=0.4834, got %f", result.Usage.Cost.Total)
	}
}
