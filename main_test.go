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
