package main

import (
	"encoding/json"
	"strings"
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

func TestExtractToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected int
	}{
		{
			name:     "no tool calls",
			content:  "plain text",
			expected: 0,
		},
		{
			name: "single tool call",
			content: []interface{}{
				map[string]interface{}{
					"type": "toolCall",
					"name": "exec",
					"arguments": map[string]interface{}{
						"command": "ls -la",
					},
				},
			},
			expected: 1,
		},
		{
			name: "multiple tool calls",
			content: []interface{}{
				map[string]interface{}{
					"type": "toolCall",
					"name": "read",
					"arguments": map[string]interface{}{
						"file_path": "/home/user/test.txt",
					},
				},
				map[string]interface{}{
					"type": "toolCall",
					"name": "exec",
					"arguments": map[string]interface{}{
						"command": "pwd",
					},
				},
			},
			expected: 2,
		},
		{
			name: "mixed content with tool call",
			content: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Let me check that",
				},
				map[string]interface{}{
					"type": "toolCall",
					"name": "read",
					"arguments": map[string]interface{}{
						"file_path": "/home/user/test.txt",
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := extractToolCalls(tt.content)
			if len(calls) != tt.expected {
				t.Errorf("Expected %d tool calls, got %d", tt.expected, len(calls))
			}
		})
	}
}

func TestExtractToolResults(t *testing.T) {
	tests := []struct {
		name     string
		content  interface{}
		expected int
	}{
		{
			name:     "no tool results",
			content:  "plain text",
			expected: 0,
		},
		{
			name: "single tool result",
			content: []interface{}{
				map[string]interface{}{
					"type": "toolResult",
					"text": "Command executed successfully",
				},
			},
			expected: 1,
		},
		{
			name: "tool result with nested content",
			content: []interface{}{
				map[string]interface{}{
					"type": "toolResult",
					"content": []interface{}{
						map[string]interface{}{
							"text": "File contents here",
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "long tool result gets truncated",
			content: []interface{}{
				map[string]interface{}{
					"type": "toolResult",
					"text": strings.Repeat("a", 400),
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := extractToolResults(tt.content)
			if len(results) != tt.expected {
				t.Errorf("Expected %d tool results, got %d", tt.expected, len(results))
			}
			// Verify truncation for long results
			if tt.name == "long tool result gets truncated" && len(results) > 0 {
				// Results include ANSI codes and prefix, but the text portion should be truncated
				if !strings.Contains(results[0], "…") {
					t.Error("Expected long result to be truncated with ellipsis")
				}
			}
		})
	}
}

func TestProcessLineWithToolCalls(t *testing.T) {
	jsonl := `{"message":{"role":"assistant","content":[{"type":"text","text":"Let me check that"},{"type":"toolCall","name":"exec","arguments":{"command":"ls -la"}}]}}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for assistant message with tool call")
	}
	
	if !strings.Contains(result.Output, "exec") {
		t.Error("Expected output to contain tool call name")
	}
	
	if !strings.Contains(result.Output, "⚡") {
		t.Error("Expected output to contain lightning bolt symbol")
	}
}

func TestProcessLineWithToolResults(t *testing.T) {
	jsonl := `{"message":{"role":"tool","content":[{"type":"toolResult","text":"total 24\ndrwxr-xr-x 3 user user 4096"}]}}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for tool message with result")
	}
	
	if !strings.Contains(result.Output, "→") {
		t.Error("Expected output to contain arrow symbol")
	}
	
	if !strings.Contains(result.Output, "total 24") {
		t.Error("Expected output to contain result text")
	}
}

func TestInberFormatUser(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:00Z","role":"user","content":"Hello, how are you?"}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber user message")
	}
	
	if !strings.Contains(result.Output, "Hello, how are you?") {
		t.Error("Expected output to contain user message")
	}
}

func TestInberFormatAssistant(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:01Z","role":"assistant","content":"I'm doing well!","model":"claude-sonnet-4","in_tokens":100,"out_tokens":20,"cost_usd":0.0123}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber assistant message")
	}
	
	if !strings.Contains(result.Output, "I'm doing well!") {
		t.Error("Expected output to contain assistant message")
	}
	
	if result.Usage == nil {
		t.Fatal("Expected usage data")
	}
	
	if result.Usage.Input != 100 {
		t.Errorf("Expected input=100, got %d", result.Usage.Input)
	}
	
	if result.Usage.Output != 20 {
		t.Errorf("Expected output=20, got %d", result.Usage.Output)
	}
	
	if result.Usage.Cost == nil || result.Usage.Cost.Total != 0.0123 {
		t.Errorf("Expected cost=0.0123, got %v", result.Usage.Cost)
	}
}

func TestInberFormatThinking(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:02Z","role":"thinking","content":"Let me think about this..."}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber thinking message")
	}
	
	if !strings.Contains(result.Output, "💭") {
		t.Error("Expected output to contain thinking emoji")
	}
	
	if !strings.Contains(result.Output, "Let me think about this...") {
		t.Error("Expected output to contain thinking content")
	}
}

func TestInberFormatToolCall(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:03Z","role":"tool_call","tool_id":"call_123","tool_name":"shell","tool_input":{"command":"ls -la"}}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber tool call")
	}
	
	if !strings.Contains(result.Output, "⚡") {
		t.Error("Expected output to contain lightning emoji")
	}
	
	if !strings.Contains(result.Output, "shell") {
		t.Error("Expected output to contain tool name")
	}
	
	if !strings.Contains(result.Output, "command") {
		t.Error("Expected output to contain tool input summary")
	}
}

func TestInberFormatToolResult(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:04Z","role":"tool_result","tool_id":"call_123","tool_name":"shell","content":"total 24\ndrwxr-xr-x 3 user user 4096","is_error":false}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber tool result")
	}
	
	if !strings.Contains(result.Output, "→") {
		t.Error("Expected output to contain arrow symbol")
	}
}

func TestInberFormatToolResultError(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:04Z","role":"tool_result","tool_id":"call_123","tool_name":"shell","content":"command not found","is_error":true}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber tool result error")
	}
	
	if !strings.Contains(result.Output, "✗") {
		t.Error("Expected output to contain error symbol")
	}
	
	if !strings.Contains(result.Output, "command not found") {
		t.Error("Expected output to contain error message")
	}
}

func TestInberFormatRequest(t *testing.T) {
	// By default, request entries should be skipped
	jsonl := `{"ts":"2024-02-24T10:30:05Z","role":"request","request":{"model":"claude-sonnet-4","messages":[]}}`
	
	verboseMode = false
	result := processLine(jsonl)
	
	if result.Output != "" {
		t.Error("Expected request entry to be skipped in non-verbose mode")
	}
	
	// In verbose mode, request entries should be shown
	verboseMode = true
	result = processLine(jsonl)
	verboseMode = false // Reset
	
	if result.Output == "" {
		t.Fatal("Expected output for request entry in verbose mode")
	}
	
	if !strings.Contains(result.Output, "[request]") {
		t.Error("Expected output to contain [request] label")
	}
}

func TestInberFormatSystem(t *testing.T) {
	jsonl := `{"ts":"2024-02-24T10:30:06Z","role":"system","content":"session started — model: claude-sonnet-4"}`
	
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for inber system message")
	}
	
	if !strings.Contains(result.Output, "[system]") {
		t.Error("Expected output to contain [system] label")
	}
	
	if !strings.Contains(result.Output, "session started") {
		t.Error("Expected output to contain system message")
	}
}

func TestProcessLineLargeJSONL(t *testing.T) {
	// Test that we can handle JSONL lines larger than 64KB
	// (the old bufio.Scanner default limit that was silently truncating)
	
	// Create a large content string (100KB to exceed the old 64KB limit)
	largeContent := strings.Repeat("This is a large tool result content. ", 100*1024/40) // ~100KB
	
	// Build a JSONL entry with the large content
	entry := map[string]interface{}{
		"message": map[string]interface{}{
			"role": "tool",
			"content": []interface{}{
				map[string]interface{}{
					"type": "toolResult",
					"text": largeContent,
				},
			},
		},
	}
	
	jsonlBytes, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal large entry: %v", err)
	}
	
	jsonl := string(jsonlBytes)
	
	// Verify the JSONL is actually larger than 64KB
	if len(jsonl) <= 64*1024 {
		t.Fatalf("Test JSONL is only %d bytes, expected > 64KB", len(jsonl))
	}
	
	// Process the line - this should not panic or truncate
	result := processLine(jsonl)
	
	if result.Output == "" {
		t.Fatal("Expected output for large tool result, got empty string")
	}
	
	// Verify the message was parsed correctly (should contain tool result marker)
	if !strings.Contains(result.Output, "→") {
		t.Error("Expected output to contain tool result arrow symbol")
	}
	
	// Now test with an assistant message with large content
	largeAssistantContent := strings.Repeat("A", 100*1024) // 100KB of 'A's
	
	assistantEntry := map[string]interface{}{
		"message": map[string]interface{}{
			"role":    "assistant",
			"content": largeAssistantContent,
			"usage": map[string]interface{}{
				"input":       1000,
				"output":      500,
				"totalTokens": 50000,
			},
		},
	}
	
	assistantJsonlBytes, err := json.Marshal(assistantEntry)
	if err != nil {
		t.Fatalf("Failed to marshal large assistant entry: %v", err)
	}
	
	assistantJsonl := string(assistantJsonlBytes)
	
	// Verify this is also > 64KB
	if len(assistantJsonl) <= 64*1024 {
		t.Fatalf("Test assistant JSONL is only %d bytes, expected > 64KB", len(assistantJsonl))
	}
	
	// Process the large assistant message
	assistantResult := processLine(assistantJsonl)
	
	if assistantResult.Output == "" {
		t.Fatal("Expected output for large assistant message, got empty string")
	}
	
	// Verify usage was extracted correctly even from large message
	if assistantResult.Usage == nil {
		t.Fatal("Expected usage data to be extracted from large assistant message")
	}
	
	if assistantResult.Usage.TotalTokens != 50000 {
		t.Errorf("Expected totalTokens=50000, got %d", assistantResult.Usage.TotalTokens)
	}
}
