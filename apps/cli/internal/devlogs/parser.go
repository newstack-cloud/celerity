package devlogs

import (
	"encoding/json"
	"strings"
)

const (
	LevelDebug  = "DEBUG"
	LevelInfo   = "INFO"
	LevelWarn   = "WARN"
	LevelError  = "ERROR"
	LevelSystem = "SYSTEM"
)

// LogLine is a parsed container log line with extracted metadata.
type LogLine struct {
	Timestamp   string
	Level       string
	Message     string
	HandlerName string
	RequestID   string
	// Source identifies the origin of the log line. "runtime" indicates a
	// Rust core runtime log (tracing JSON); empty for SDK/application logs.
	Source  string
	RawLine string
	Extra   map[string]any
}

// SDKLogParseFunc attempts to parse a raw log line in an SDK-specific format.
type SDKLogParseFunc func(raw string) (LogLine, bool)

// LogParser dispatches to runtime-specific SDK parsers and falls back
// to plain-text passthrough for unrecognised lines.
type LogParser struct {
	sdkParser SDKLogParseFunc
}

// NewLogParser creates a parser configured for the given application runtime.
func NewLogParser(runtime string) *LogParser {
	return &LogParser{sdkParser: sdkParserForRuntime(runtime)}
}

// Parse extracts structured metadata from a raw log line.
// Order: tracing JSON → SDK parser → plain-text passthrough.
func (p *LogParser) Parse(raw string) LogLine {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return LogLine{RawLine: raw, Level: LevelInfo}
	}

	if trimmed[0] == '{' {
		return p.parseJSON(trimmed)
	}

	if p.sdkParser != nil {
		if line, ok := p.sdkParser(trimmed); ok {
			return line
		}
	}

	return LogLine{RawLine: raw, Level: LevelInfo, Message: raw}
}

func (p *LogParser) parseJSON(trimmed string) LogLine {
	if line, ok := parseTracingJSON(trimmed); ok {
		return line
	}
	if p.sdkParser != nil {
		if line, ok := p.sdkParser(trimmed); ok {
			return line
		}
	}
	return LogLine{RawLine: trimmed, Level: LevelInfo, Message: trimmed}
}

func sdkParserForRuntime(runtime string) SDKLogParseFunc {
	lower := strings.ToLower(runtime)
	switch {
	case strings.HasPrefix(lower, "nodejs") || lower == "node":
		return parseNodeJSLog
	case strings.HasPrefix(lower, "python"):
		return parseStructlogJSON
	default:
		return nil
	}
}

func parseNodeJSLog(raw string) (LogLine, bool) {
	if len(raw) == 0 {
		return LogLine{}, false
	}
	if isNodeJSDebugLine(raw) {
		return LogLine{Level: LevelSystem, Message: raw, RawLine: raw}, true
	}
	if raw[0] != '{' {
		return LogLine{}, false
	}
	return parsePinoJSON(raw)
}

// isNodeJSDebugLine reports whether raw looks like output from the Node.js
// debug package, which prefixes every line with an ISO 8601 timestamp
// (YYYY-MM-DDTHH:MM:SS...) followed by the debug namespace and message.
func isNodeJSDebugLine(raw string) bool {
	if len(raw) < 20 {
		return false
	}
	return raw[4] == '-' && raw[7] == '-' && raw[10] == 'T' && raw[13] == ':' && raw[16] == ':'
}

// --- pino JSON ---

var pinoKnownKeys = map[string]bool{
	"level": true, "time": true, "msg": true,
	"handlerName": true, "context": true, "requestId": true,
	"pid": true, "hostname": true, "v": true, "name": true,
}

func parsePinoJSON(raw string) (LogLine, bool) {
	var all map[string]any
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return LogLine{}, false
	}

	levelVal, hasLevel := all["level"]
	msgVal, hasMsg := all["msg"]
	if !hasLevel && !hasMsg {
		return LogLine{}, false
	}

	levelNum, isNum := levelVal.(float64)
	if hasLevel && !isNum {
		return LogLine{}, false
	}

	msg, _ := msgVal.(string)
	line := LogLine{
		Timestamp:   stringFromMap(all, "time"),
		Level:       pinoLevelToString(int(levelNum)),
		Message:     msg,
		HandlerName: stringFromMap(all, "handlerName"),
		RequestID:   stringFromMap(all, "requestId"),
		RawLine:     raw,
		Extra:       collectExtra(all, pinoKnownKeys),
	}

	if line.HandlerName == "" {
		line.HandlerName = stringFromMap(all, "context")
	}
	return line, true
}

func pinoLevelToString(level int) string {
	switch {
	case level <= 20:
		return LevelDebug
	case level <= 30:
		return LevelInfo
	case level <= 40:
		return LevelWarn
	default:
		return LevelError
	}
}

// --- structlog JSON (Python SDK) ---

var structlogKnownKeys = map[string]bool{
	"msg": true, "level": true, "timestamp": true,
	"logger_name": true, "request_id": true,
	"method": true, "path": true, "matched_route": true,
	"client_ip": true, "user_agent": true,
	"source": true, "message_count": true,
}

var structlogLevels = map[string]string{
	"debug":   LevelDebug,
	"info":    LevelInfo,
	"warning": LevelWarn,
	"error":   LevelError,
}

func parseStructlogJSON(raw string) (LogLine, bool) {
	if len(raw) == 0 || raw[0] != '{' {
		return LogLine{}, false
	}

	var all map[string]any
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return LogLine{}, false
	}

	levelStr, hasLevel := all["level"].(string)
	if !hasLevel {
		return LogLine{}, false
	}
	level, validLevel := structlogLevels[levelStr]
	if !validLevel {
		return LogLine{}, false
	}

	line := LogLine{
		Timestamp:   stringFromMap(all, "timestamp"),
		Level:       level,
		Message:     stringFromMap(all, "msg"),
		HandlerName: stringFromMap(all, "matched_route"),
		RequestID:   stringFromMap(all, "request_id"),
		RawLine:     raw,
		Extra:       collectExtra(all, structlogKnownKeys),
	}

	return line, true
}

// --- tracing JSON ---

var tracingKnownKeys = map[string]bool{
	"timestamp": true, "level": true, "fields": true,
	"target": true, "span": true, "spans": true,
	"threadName": true, "threadId": true,
}

var tracingFieldKnownKeys = map[string]bool{
	"message": true,
}

var validTracingLevels = map[string]bool{
	LevelDebug: true, LevelInfo: true,
	LevelWarn: true, LevelError: true,
}

type tracingJSONSpan struct {
	HandlerName string
	RequestID   string
	Name        string
}

func parseTracingJSON(raw string) (LogLine, bool) {
	var all map[string]any
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return LogLine{}, false
	}

	if _, hasFields := all["fields"]; !hasFields {
		return LogLine{}, false
	}

	levelStr, _ := all["level"].(string)
	level := strings.ToUpper(levelStr)
	if !validTracingLevels[level] {
		return LogLine{}, false
	}

	message, extra := extractTracingFields(all)

	line := LogLine{
		Timestamp: stringFromMap(all, "timestamp"),
		Level:     level,
		Message:   message,
		Source:    "runtime",
		RawLine:   raw,
		Extra:     extra,
	}

	populateSpanFields(&line, all)
	return line, true
}

func extractTracingFields(all map[string]any) (string, map[string]any) {
	extra := make(map[string]any)
	message := ""

	if fields, ok := all["fields"].(map[string]any); ok {
		if msg, ok := fields["message"].(string); ok {
			message = msg
		}
		for k, v := range fields {
			if !tracingFieldKnownKeys[k] {
				extra[k] = v
			}
		}
	}

	for k, v := range all {
		if !tracingKnownKeys[k] {
			extra[k] = v
		}
	}

	if len(extra) == 0 {
		return message, nil
	}
	return message, extra
}

func populateSpanFields(line *LogLine, all map[string]any) {
	if spanRaw, ok := all["span"]; ok {
		if span := parseSpanMap(spanRaw); span != nil {
			line.HandlerName = span.HandlerName
			line.RequestID = span.RequestID
		}
	}

	spansRaw, ok := all["spans"].([]any)
	if !ok {
		return
	}
	for _, s := range spansRaw {
		span := parseSpanMap(s)
		if span == nil {
			continue
		}
		if line.HandlerName == "" && span.HandlerName != "" {
			line.HandlerName = span.HandlerName
		}
		if line.RequestID == "" && span.RequestID != "" {
			line.RequestID = span.RequestID
		}
	}
}

func parseSpanMap(v any) *tracingJSONSpan {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return &tracingJSONSpan{
		HandlerName: stringFromMap(m, "handler_name"),
		RequestID:   stringFromMap(m, "request_id"),
		Name:        stringFromMap(m, "name"),
	}
}

// --- helpers ---

func stringFromMap(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func collectExtra(all map[string]any, known map[string]bool) map[string]any {
	extra := make(map[string]any)
	for k, v := range all {
		if !known[k] {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

// --- filtering ---

// MatchesHandler checks if the handler name contains the filter substring.
func (l *LogLine) MatchesHandler(filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(
		strings.ToLower(l.HandlerName),
		strings.ToLower(filter),
	)
}

// MatchesLevel checks if the log line meets the minimum level threshold.
func (l *LogLine) MatchesLevel(minLevel string) bool {
	if minLevel == "" {
		return true
	}
	return levelRank(l.Level) >= levelRank(strings.ToUpper(minLevel))
}

var levelRanks = map[string]int{
	LevelDebug: 0, LevelInfo: 1,
	LevelWarn: 2, LevelError: 3,
	LevelSystem: 4,
}

func levelRank(level string) int {
	if r, ok := levelRanks[level]; ok {
		return r
	}
	return 0
}
