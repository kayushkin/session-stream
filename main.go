package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ANSI color codes
const (
	cyan    = "\033[36m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	red     = "\033[31m"
	dim     = "\033[2m"
	bold    = "\033[1m"
	reset   = "\033[0m"
	magenta = "\033[35m"
	blue    = "\033[34m"
)

const (
	defaultAgent = "main"
	defaultTail  = 20
)

// Message structures
type Cost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

type Usage struct {
	Input       int   `json:"input"`
	Output      int   `json:"output"`
	CacheRead   int   `json:"cacheRead"`
	CacheWrite  int   `json:"cacheWrite"`
	TotalTokens int   `json:"totalTokens"`
	Cost        *Cost `json:"cost"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
	Usage   *Usage      `json:"usage"`
}

type LogEntry struct {
	// OpenClaw format
	Message   Message     `json:"message"`
	Timestamp interface{} `json:"timestamp"`
	
	// Inber format (flat structure)
	Role       string                 `json:"role"`
	Content    interface{}            `json:"content"`
	TS         string                 `json:"ts"`
	Model      string                 `json:"model"`
	InTokens   int                    `json:"in_tokens"`
	OutTokens  int                    `json:"out_tokens"`
	CostUSD    float64                `json:"cost_usd"`
	ToolID     string                 `json:"tool_id"`
	ToolName   string                 `json:"tool_name"`
	ToolInput  map[string]interface{} `json:"tool_input"`
	IsError    bool                   `json:"is_error"`
	Request    map[string]interface{} `json:"request"`
}

// Global verbose flag
var verboseMode bool

type ContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Content   interface{}            `json:"content"`
}

func getStateDir() string {
	stateDir := os.Getenv("OPENCLAW_STATE_DIR")
	if stateDir == "" {
		home, _ := os.UserHomeDir()
		stateDir = filepath.Join(home, ".openclaw")
	}
	return stateDir
}

func getAgentsDir() string {
	return filepath.Join(getStateDir(), "agents")
}

type AgentInfo struct {
	Name  string
	Count int
}

func getAgents() []AgentInfo {
	agentsDir := getAgentsDir()
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return []AgentInfo{}
	}

	var agents []AgentInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionsDir := filepath.Join(agentsDir, entry.Name(), "sessions")
		if info, err := os.Stat(sessionsDir); err == nil && info.IsDir() {
			pattern := filepath.Join(sessionsDir, "*.jsonl")
			matches, _ := filepath.Glob(pattern)
			agents = append(agents, AgentInfo{Name: entry.Name(), Count: len(matches)})
		}
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents
}

type SessionFile struct {
	Path    string
	ModTime time.Time
}

func getSessions(agent string) []SessionFile {
	pattern := filepath.Join(getAgentsDir(), agent, "sessions", "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return []SessionFile{}
	}

	var sessions []SessionFile
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, SessionFile{Path: path, ModTime: info.ModTime()})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})
	return sessions
}

func findLatestSession(agent string) string {
	sessions := getSessions(agent)
	if len(sessions) == 0 {
		fmt.Fprintf(os.Stderr, "%sNo session files found for agent '%s'%s\n", red, agent, reset)
		fmt.Fprintf(os.Stderr, "%sLooked in: %s/%s/sessions/*.jsonl%s\n", dim, getAgentsDir(), agent, reset)
		agents := getAgents()
		if len(agents) > 0 {
			var names []string
			for _, a := range agents {
				names = append(names, a.Name)
			}
			fmt.Fprintf(os.Stderr, "\nAvailable agents: %s\n", strings.Join(names, ", "))
		}
		os.Exit(1)
	}
	return sessions[0].Path
}

func extractText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, block := range v {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			} else if str, ok := block.(string); ok {
				parts = append(parts, str)
			}
		}
		return strings.Join(parts, "\n")
	default:
		if v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
}

func extractToolCalls(content interface{}) []string {
	var calls []string
	contentSlice, ok := content.([]interface{})
	if !ok {
		return calls
	}

	for _, block := range contentSlice {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if blockMap["type"] != "toolCall" {
			continue
		}

		name := "?"
		if n, ok := blockMap["name"].(string); ok {
			name = n
		}

		argsStr := ""
		if args, ok := blockMap["arguments"].(map[string]interface{}); ok {
			var summary []string
			for k, v := range args {
				vStr := fmt.Sprintf("%v", v)
				if len(vStr) > 80 {
					vStr = vStr[:77] + "…"
				}
				summary = append(summary, fmt.Sprintf("%s=%s", k, vStr))
			}
			argsStr = strings.Join(summary, ", ")
		} else if args, ok := blockMap["arguments"]; ok {
			argsStr = fmt.Sprintf("%v", args)
			if len(argsStr) > 150 {
				argsStr = argsStr[:150]
			}
		}

		calls = append(calls, fmt.Sprintf("  %s⚡ %s%s(%s%s%s)", magenta, name, reset, dim, argsStr, reset))
	}
	return calls
}

func extractToolResults(content interface{}) []string {
	var results []string
	contentSlice, ok := content.([]interface{})
	if !ok {
		return results
	}

	for _, block := range contentSlice {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if blockMap["type"] != "toolResult" {
			continue
		}

		text := ""
		if t, ok := blockMap["text"].(string); ok {
			text = t
		} else if c, ok := blockMap["content"]; ok {
			if cSlice, ok := c.([]interface{}); ok {
				var parts []string
				for _, item := range cSlice {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if t, ok := itemMap["text"].(string); ok {
							parts = append(parts, t)
						}
					}
				}
				text = strings.Join(parts, " ")
			} else {
				text = fmt.Sprintf("%v", c)
			}
		}

		if len(text) > 300 {
			text = text[:297] + "…"
		}
		if strings.TrimSpace(text) != "" {
			results = append(results, fmt.Sprintf("  %s→ %s%s", dim, text, reset))
		}
	}
	return results
}

func formatNumber(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func formatTokenUsage(usage *Usage) string {
	if usage == nil || usage.TotalTokens == 0 && usage.Output == 0 {
		return ""
	}
	costStr := ""
	if usage.Cost != nil && usage.Cost.Total > 0 {
		costStr = fmt.Sprintf(" | %s", formatCost(usage.Cost.Total))
	}
	return fmt.Sprintf(" %sctx: %s | out: %d%s%s", dim, formatNumber(usage.TotalTokens), usage.Output, costStr, reset)
}

func formatTimestamp(entry *LogEntry) string {
	var ts interface{}
	if entry.TS != "" {
		ts = entry.TS
	} else if entry.Timestamp != nil {
		ts = entry.Timestamp
	} else {
		return ""
	}

	switch v := ts.(type) {
	case float64:
		t := v
		if t > 1e12 {
			t = t / 1000
		}
		return fmt.Sprintf(" %s%s%s", dim, time.Unix(int64(t), 0).Format("15:04:05"), reset)
	case string:
		return fmt.Sprintf(" %s%s%s", dim, v, reset)
	default:
		return ""
	}
}

type ProcessedLine struct {
	Output string
	Usage  *Usage
}

// normalizeEntry converts an inber format entry to OpenClaw Message format
func normalizeEntry(entry *LogEntry) (string, interface{}, *Usage, interface{}) {
	// Check if this is inber format (has role at top level)
	if entry.Role != "" && entry.Message.Role == "" {
		// Inber format
		role := entry.Role
		content := entry.Content
		ts := entry.TS
		
		// Build usage from inber fields
		var usage *Usage
		if entry.InTokens > 0 || entry.OutTokens > 0 {
			usage = &Usage{
				Input:       entry.InTokens,
				Output:      entry.OutTokens,
				TotalTokens: entry.InTokens,
			}
			if entry.CostUSD > 0 {
				usage.Cost = &Cost{
					Total: entry.CostUSD,
				}
			}
		}
		
		return role, content, usage, ts
	}
	
	// OpenClaw format
	var ts interface{}
	if entry.TS != "" {
		ts = entry.TS
	} else if entry.Timestamp != nil {
		ts = entry.Timestamp
	}
	return entry.Message.Role, entry.Message.Content, entry.Message.Usage, ts
}

func processLine(line string) ProcessedLine {
	line = strings.TrimSpace(line)
	if line == "" {
		return ProcessedLine{}
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return ProcessedLine{}
	}

	role, content, usage, tsValue := normalizeEntry(&entry)
	
	if role == "" {
		return ProcessedLine{}
	}

	// Format timestamp
	ts := ""
	switch v := tsValue.(type) {
	case string:
		if v != "" {
			// Try parsing as RFC3339 first (inber format)
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				ts = fmt.Sprintf(" %s%s%s", dim, t.Format("15:04:05"), reset)
			} else if t, err := time.Parse(time.RFC3339, v); err == nil {
				ts = fmt.Sprintf(" %s%s%s", dim, t.Format("15:04:05"), reset)
			} else {
				ts = fmt.Sprintf(" %s%s%s", dim, v, reset)
			}
		}
	case float64:
		// Unix timestamp (OpenClaw format)
		t := v
		if t > 1e12 {
			t = t / 1000
		}
		ts = fmt.Sprintf(" %s%s%s", dim, time.Unix(int64(t), 0).Format("15:04:05"), reset)
	}

	switch role {
	case "request":
		// Inber format: full API request payload
		// Skip by default unless verbose mode is enabled
		if verboseMode {
			return ProcessedLine{
				Output: fmt.Sprintf("\n%s%s[request]%s%s%s", blue, dim, ts, reset, ""),
			}
		}
		return ProcessedLine{}
	
	case "thinking":
		// Inber format: reasoning text
		text := extractText(content)
		if text != "" {
			if len(text) > 500 {
				text = text[:200] + fmt.Sprintf("\n  %s… (%d chars)%s", dim, len(text), reset)
			}
			return ProcessedLine{
				Output: fmt.Sprintf("\n%s%s💭 Thinking%s ━━━%s\n%s%s%s", yellow, bold, ts, reset, dim, text, reset),
			}
		}
	
	case "tool_call":
		// Inber format: individual tool call
		name := entry.ToolName
		if name == "" {
			name = "?"
		}
		
		argsStr := ""
		if len(entry.ToolInput) > 0 {
			var summary []string
			for k, v := range entry.ToolInput {
				vStr := fmt.Sprintf("%v", v)
				if len(vStr) > 80 {
					vStr = vStr[:77] + "…"
				}
				summary = append(summary, fmt.Sprintf("%s=%s", k, vStr))
			}
			argsStr = strings.Join(summary, ", ")
		}
		
		return ProcessedLine{
			Output: fmt.Sprintf("  %s⚡ %s%s(%s%s%s)", magenta, name, reset, dim, argsStr, reset),
		}
	
	case "tool_result":
		// Inber format: individual tool result
		text := extractText(content)
		if entry.IsError {
			if len(text) > 300 {
				text = text[:297] + "…"
			}
			return ProcessedLine{
				Output: fmt.Sprintf("  %s✗ %s%s", red, text, reset),
			}
		}
		
		// Count lines/bytes for non-error results
		lineCount := strings.Count(text, "\n") + 1
		byteCount := len(text)
		if byteCount > 0 {
			if lineCount == 1 && byteCount < 100 {
				return ProcessedLine{
					Output: fmt.Sprintf("  %s→ %s%s", dim, text, reset),
				}
			}
			return ProcessedLine{
				Output: fmt.Sprintf("  %s→ %d lines, %d bytes%s", dim, lineCount, byteCount, reset),
			}
		}
		return ProcessedLine{}

	case "user":
		text := extractText(content)
		if text != "" && !strings.HasPrefix(text, "Read HEARTBEAT") {
			if len(text) > 500 {
				text = text[:200] + fmt.Sprintf("\n  %s… (%d chars)%s", dim, len(text), reset)
			}
			return ProcessedLine{
				Output: fmt.Sprintf("\n%s%s━━━ You%s ━━━%s\n%s%s%s", cyan, bold, ts, reset, cyan, text, reset),
			}
		}

	case "assistant":
		var parts []string
		text := extractText(content)
		tokens := formatTokenUsage(usage)
		if strings.TrimSpace(text) != "" {
			parts = append(parts, fmt.Sprintf("\n%s%s━━━ Agent%s%s ━━━%s\n%s%s%s", green, bold, ts, tokens, reset, green, text, reset))
		}
		toolCalls := extractToolCalls(content)
		if len(toolCalls) > 0 {
			if len(parts) == 0 {
				parts = append(parts, fmt.Sprintf("\n%s%s━━━ Agent%s%s ━━━%s", green, bold, ts, tokens, reset))
			}
			parts = append(parts, toolCalls...)
		}
		if len(parts) > 0 {
			return ProcessedLine{
				Output: strings.Join(parts, "\n"),
				Usage:  usage,
			}
		}

	case "tool":
		results := extractToolResults(content)
		if len(results) > 0 {
			return ProcessedLine{
				Output: strings.Join(results, "\n"),
			}
		}
		text := extractText(content)
		if strings.TrimSpace(text) != "" {
			if len(text) > 300 {
				text = text[:297] + "…"
			}
			return ProcessedLine{
				Output: fmt.Sprintf("  %s→ %s%s", dim, text, reset),
			}
		}

	case "system":
		text := extractText(content)
		if strings.TrimSpace(text) != "" {
			if len(text) > 200 {
				text = text[:197] + "…"
			}
			return ProcessedLine{
				Output: fmt.Sprintf("\n%s%s[system]%s %s%s", blue, dim, ts, text, reset),
			}
		}
	}

	return ProcessedLine{}
}

func streamFile(filepath string, follow bool, tail int) {
	basename := filepath[strings.LastIndex(filepath, "/")+1:]
	agentName := ""
	parts := strings.Split(filepath, "/")
	for i, p := range parts {
		if p == "agents" && i+1 < len(parts) {
			agentName = fmt.Sprintf(" (%s)", parts[i+1])
			break
		}
	}

	fmt.Printf("%sStreaming: %s%s%s\n", yellow, basename, agentName, reset)
	fmt.Printf("%s%s%s\n\n", dim, strings.Repeat("─", 60), reset)

	file, err := os.Open(filepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError opening file: %v%s\n", red, err, reset)
		os.Exit(1)
	}
	defer file.Close()

	// Read all existing lines
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // up to 10MB per line
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Track token usage and cost
	var totalContext, totalOutput int
	var totalCost float64

	// Print tail
	start := 0
	if follow && len(lines) > tail {
		start = len(lines) - tail
	}
	for _, line := range lines[start:] {
		result := processLine(line)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		if result.Usage != nil {
			totalContext += result.Usage.TotalTokens
			totalOutput += result.Usage.Output
			if result.Usage.Cost != nil {
				totalCost += result.Usage.Cost.Total
			}
		}
	}

	if !follow {
		// Show total when dumping
		if totalContext > 0 || totalOutput > 0 {
			fmt.Printf("\n%s%s%s\n", dim, strings.Repeat("─", 60), reset)
			costStr := ""
			if totalCost > 0 {
				costStr = fmt.Sprintf(" | %s", formatCost(totalCost))
			}
			fmt.Printf("%sTotal: ctx: %s | out: %s%s%s\n", dim, formatNumber(totalContext), formatNumber(totalOutput), costStr, reset)
		}
		return
	}

	// Follow mode
	for {
		line, err := readLine(file)
		if err == io.EOF {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err != nil {
			break
		}
		result := processLine(line)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		if result.Usage != nil {
			totalContext += result.Usage.TotalTokens
			totalOutput += result.Usage.Output
			if result.Usage.Cost != nil {
				totalCost += result.Usage.Cost.Total
			}
		}
	}
}

func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

func listAgents() {
	agents := getAgents()
	if len(agents) == 0 {
		fmt.Fprintf(os.Stderr, "%sNo agents found in %s%s\n", red, getAgentsDir(), reset)
		os.Exit(1)
	}
	fmt.Printf("%sAgents:%s\n\n", bold, reset)
	for _, agent := range agents {
		fmt.Printf("  %s%s%s  %s(%d sessions)%s\n", cyan, agent.Name, reset, dim, agent.Count, reset)
	}
}

func listSessions(agent string) {
	sessions := getSessions(agent)
	if len(sessions) == 0 {
		fmt.Fprintf(os.Stderr, "%sNo sessions for agent '%s'%s\n", red, agent, reset)
		os.Exit(1)
	}
	fmt.Printf("%sSessions for %s%s%s%s:%s\n\n", bold, cyan, agent, reset, bold, reset)

	limit := 20
	if len(sessions) < limit {
		limit = len(sessions)
	}
	for _, session := range sessions[:limit] {
		basename := filepath.Base(session.Path)
		info, _ := os.Stat(session.Path)
		size := info.Size()
		sizeStr := fmt.Sprintf("%.0fK", float64(size)/1024)
		if size >= 1024*1024 {
			sizeStr = fmt.Sprintf("%.1fM", float64(size)/(1024*1024))
		}
		mtime := session.ModTime.Format("2006-01-02 15:04")
		fmt.Printf("  %s%s%s  %6s  %s\n", dim, mtime, reset, sizeStr, basename)
	}
}

func main() {
	agent := flag.String("agent", defaultAgent, "Agent id")
	flag.StringVar(agent, "a", defaultAgent, "Agent id (shorthand)")
	list := flag.Bool("list", false, "List agents or sessions")
	flag.BoolVar(list, "l", false, "List agents or sessions (shorthand)")
	noFollow := flag.Bool("no-follow", false, "Dump and exit")
	n := flag.Int("n", defaultTail, "Number of recent messages to show")
	verbose := flag.Bool("verbose", false, "Show request entries (inber format)")
	flag.BoolVar(verbose, "v", false, "Show request entries (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Stream OpenClaw and inber session logs in a readable format.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  session-stream                        # latest session for default agent (main)\n")
		fmt.Fprintf(os.Stderr, "  session-stream --agent argraphments   # latest session for a specific agent\n")
		fmt.Fprintf(os.Stderr, "  session-stream --list                 # list available agents\n")
		fmt.Fprintf(os.Stderr, "  session-stream --list --agent work    # list sessions for an agent\n")
		fmt.Fprintf(os.Stderr, "  session-stream <path>.jsonl           # stream a specific file\n")
		fmt.Fprintf(os.Stderr, "  session-stream --no-follow            # dump and exit (no tail)\n")
		fmt.Fprintf(os.Stderr, "  session-stream -n 50                  # show last N messages instead of default 20\n")
		fmt.Fprintf(os.Stderr, "  session-stream --verbose              # show request entries (inber format)\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()
	
	// Set global verbose flag
	verboseMode = *verbose

	if *list {
		if *agent != defaultAgent {
			listSessions(*agent)
		} else {
			listAgents()
		}
		return
	}

	filepath := ""
	if flag.NArg() > 0 {
		filepath = flag.Arg(0)
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%sFile not found: %s%s\n", red, filepath, reset)
			os.Exit(1)
		}
	} else {
		filepath = findLatestSession(*agent)
	}

	streamFile(filepath, !*noFollow, *n)
}
