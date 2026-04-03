package devlogs

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ParserTestSuite struct {
	suite.Suite
}

func (s *ParserTestSuite) nodeParser() *LogParser {
	return NewLogParser("nodejs")
}

func (s *ParserTestSuite) defaultParser() *LogParser {
	return NewLogParser("")
}

// --- pino JSON (via nodejs parser) ---

func (s *ParserTestSuite) Test_pino_json_info() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"hello world","handlerName":"TestHandler"}`
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal("hello world", line.Message)
	s.Equal("TestHandler", line.HandlerName)
	s.Equal("2024-01-01T00:00:00Z", line.Timestamp)
}

func (s *ParserTestSuite) Test_pino_json_error() {
	raw := `{"level":50,"time":"2024-01-01T00:00:00Z","msg":"something broke","handlerName":"Orders"}`
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelError, line.Level)
	s.Equal("something broke", line.Message)
	s.Equal("Orders", line.HandlerName)
}

func (s *ParserTestSuite) Test_pino_json_context_fallback() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"request","context":"HelloController"}`
	line := s.nodeParser().Parse(raw)
	s.Equal("HelloController", line.HandlerName)
}

func (s *ParserTestSuite) Test_pino_json_request_id() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"handled","requestId":"req-123"}`
	line := s.nodeParser().Parse(raw)
	s.Equal("req-123", line.RequestID)
}

func (s *ParserTestSuite) Test_pino_json_extra_fields() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"order created","handlerName":"Orders","orderId":"ord-1","userId":"u-abc"}`
	line := s.nodeParser().Parse(raw)
	s.Equal("order created", line.Message)
	s.Require().NotNil(line.Extra)
	s.Equal("ord-1", line.Extra["orderId"])
	s.Equal("u-abc", line.Extra["userId"])
}

func (s *ParserTestSuite) Test_pino_json_no_extra_when_only_known_fields() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"hello","pid":1234,"hostname":"test"}`
	line := s.nodeParser().Parse(raw)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_pino_json_not_matched_by_default_parser() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"hello"}`
	line := s.defaultParser().Parse(raw)
	// Default parser has no SDK parser, pino is not tried.
	// Falls through to passthrough.
	s.Equal(LevelInfo, line.Level)
	s.Equal(raw, line.Message)
}

func (s *ParserTestSuite) Test_pino_json_matched_by_versioned_nodejs_runtime() {
	// Runtimes are stored as e.g. "nodejs24.x" — the parser must use prefix matching.
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"hello","handlerName":"Orders"}`
	parser := NewLogParser("nodejs24.x")
	line := parser.Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal("hello", line.Message)
	s.Equal("Orders", line.HandlerName)
}

func (s *ParserTestSuite) Test_pino_json_name_field_excluded_from_extra() {
	// "name" is pino's logger name — should not appear as an extra field.
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"guard action","name":"guard","handlerName":"Orders"}`
	line := s.nodeParser().Parse(raw)
	s.Equal("guard action", line.Message)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_nodejs_debug_line_detected_as_system() {
	raw := "2026-02-21T12:08:36.754Z celerity:core:runtime-entry guard admin — input method=POST path=/users"
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelSystem, line.Level)
	s.Equal(raw, line.Message)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_nodejs_debug_line_not_matched_by_non_nodejs_parser() {
	raw := "2026-02-21T12:08:36.754Z celerity:core:runtime-entry some message"
	line := s.defaultParser().Parse(raw)
	// Default parser has no SDK parser; falls through to plain-text passthrough.
	s.Equal(LevelInfo, line.Level)
	s.Equal(raw, line.Message)
}

func (s *ParserTestSuite) Test_plain_text_not_confused_for_debug_line() {
	raw := "Starting Celerity Node.js runtime v1.0"
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.NotEqual(LevelSystem, line.Level)
}

// --- structlog JSON (via python parser) ---

func (s *ParserTestSuite) pythonParser() *LogParser {
	return NewLogParser("python3.13")
}

func (s *ParserTestSuite) Test_structlog_json_info() {
	raw := `{"msg":"request handled","level":"info","timestamp":"2026-03-15T10:30:00Z","logger_name":"celerity","request_id":"req-42","matched_route":"/orders/{id}"}`
	line := s.pythonParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal("request handled", line.Message)
	s.Equal("2026-03-15T10:30:00Z", line.Timestamp)
	s.Equal("/orders/{id}", line.HandlerName)
	s.Equal("req-42", line.RequestID)
}

func (s *ParserTestSuite) Test_structlog_json_warning_level() {
	raw := `{"msg":"slow query","level":"warning","timestamp":"2026-03-15T10:30:00Z"}`
	line := s.pythonParser().Parse(raw)
	s.Equal(LevelWarn, line.Level)
	s.Equal("slow query", line.Message)
}

func (s *ParserTestSuite) Test_structlog_json_error_level() {
	raw := `{"msg":"connection failed","level":"error","timestamp":"2026-03-15T10:30:00Z"}`
	line := s.pythonParser().Parse(raw)
	s.Equal(LevelError, line.Level)
}

func (s *ParserTestSuite) Test_structlog_json_debug_level() {
	raw := `{"msg":"entering handler","level":"debug","timestamp":"2026-03-15T10:30:00Z"}`
	line := s.pythonParser().Parse(raw)
	s.Equal(LevelDebug, line.Level)
}

func (s *ParserTestSuite) Test_structlog_json_extra_fields() {
	raw := `{"msg":"order created","level":"info","timestamp":"2026-03-15T10:30:00Z","order_id":"ord-1","total":99.95}`
	line := s.pythonParser().Parse(raw)
	s.Equal("order created", line.Message)
	s.Require().NotNil(line.Extra)
	s.Equal("ord-1", line.Extra["order_id"])
	s.Equal(99.95, line.Extra["total"])
}

func (s *ParserTestSuite) Test_structlog_json_no_extra_when_only_known_fields() {
	raw := `{"msg":"hello","level":"info","timestamp":"2026-03-15T10:30:00Z","logger_name":"celerity"}`
	line := s.pythonParser().Parse(raw)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_structlog_json_http_context_fields_excluded_from_extra() {
	raw := `{"msg":"handled","level":"info","timestamp":"2026-03-15T10:30:00Z","request_id":"r-1","method":"POST","path":"/users","matched_route":"/users","client_ip":"127.0.0.1","user_agent":"curl/8.0"}`
	line := s.pythonParser().Parse(raw)
	s.Equal("r-1", line.RequestID)
	s.Equal("/users", line.HandlerName)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_structlog_json_not_matched_by_default_parser() {
	raw := `{"msg":"hello","level":"info","timestamp":"2026-03-15T10:30:00Z"}`
	line := s.defaultParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal(raw, line.Message)
}

func (s *ParserTestSuite) Test_structlog_json_not_matched_by_node_parser() {
	raw := `{"msg":"hello","level":"info","timestamp":"2026-03-15T10:30:00Z"}`
	line := s.nodeParser().Parse(raw)
	// pino rejects string levels, falls through to passthrough.
	s.Equal(LevelInfo, line.Level)
	s.Equal(raw, line.Message)
}

func (s *ParserTestSuite) Test_structlog_json_consumer_context_fields() {
	raw := `{"msg":"processing batch","level":"info","timestamp":"2026-03-15T10:30:00Z","source":"order-events","message_count":5}`
	line := s.pythonParser().Parse(raw)
	s.Equal("processing batch", line.Message)
	s.Nil(line.Extra)
}

// --- tracing JSON ---

func (s *ParserTestSuite) Test_tracing_json_valid() {
	raw := `{"timestamp":"2026-02-19T14:23:01.123456Z","level":"INFO","fields":{"message":"HTTP request received"},"target":"celerity_runtime_core::telemetry"}`
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal("HTTP request received", line.Message)
	s.Equal("2026-02-19T14:23:01.123456Z", line.Timestamp)
	s.Equal("runtime", line.Source)
}

func (s *ParserTestSuite) Test_tracing_json_with_spans() {
	raw := `{"timestamp":"2026-02-19T14:23:01Z","level":"INFO","fields":{"message":"processed"},"spans":[{"name":"http_request","handler_name":"orders","request_id":"req-1"}]}`
	line := s.defaultParser().Parse(raw)
	s.Equal("orders", line.HandlerName)
	s.Equal("req-1", line.RequestID)
}

func (s *ParserTestSuite) Test_tracing_json_extra_fields_in_fields() {
	raw := `{"timestamp":"2026-02-19T14:23:01Z","level":"WARN","fields":{"message":"slow query","duration_ms":450,"query":"SELECT *"}}`
	line := s.defaultParser().Parse(raw)
	s.Equal(LevelWarn, line.Level)
	s.Equal("slow query", line.Message)
	s.Require().NotNil(line.Extra)
	s.Equal(float64(450), line.Extra["duration_ms"])
	s.Equal("SELECT *", line.Extra["query"])
}

func (s *ParserTestSuite) Test_tracing_json_rejects_pino() {
	raw := `{"level":30,"time":"2024-01-01T00:00:00Z","msg":"hello"}`
	_, ok := parseTracingJSON(raw)
	s.False(ok, "pino JSON (numeric level) should not match tracing JSON parser")
}

func (s *ParserTestSuite) Test_tracing_json_rejects_invalid() {
	_, ok := parseTracingJSON("not json at all")
	s.False(ok)
}

// --- plain text passthrough ---

func (s *ParserTestSuite) Test_plain_text_passthrough() {
	raw := "Starting Celerity Node.js runtime"
	line := s.nodeParser().Parse(raw)
	s.Equal(LevelInfo, line.Level)
	s.Equal(raw, line.Message)
	s.Nil(line.Extra)
}

func (s *ParserTestSuite) Test_empty_string() {
	line := s.nodeParser().Parse("")
	s.Equal(LevelInfo, line.Level)
	s.Equal("", line.RawLine)
}

// --- filtering ---

func (s *ParserTestSuite) Test_MatchesHandler_case_insensitive() {
	line := LogLine{HandlerName: "TestHelloHandler"}
	s.True(line.MatchesHandler("hello"))
	s.True(line.MatchesHandler("HELLO"))
	s.False(line.MatchesHandler("orders"))
}

func (s *ParserTestSuite) Test_MatchesHandler_empty_filter() {
	line := LogLine{HandlerName: "anything"}
	s.True(line.MatchesHandler(""))
}

func (s *ParserTestSuite) Test_MatchesLevel_info_threshold() {
	debug := LogLine{Level: LevelDebug}
	info := LogLine{Level: LevelInfo}
	warn := LogLine{Level: LevelWarn}
	errLine := LogLine{Level: LevelError}

	s.False(debug.MatchesLevel("info"))
	s.True(info.MatchesLevel("info"))
	s.True(warn.MatchesLevel("info"))
	s.True(errLine.MatchesLevel("info"))
}

func (s *ParserTestSuite) Test_MatchesLevel_empty_filter() {
	line := LogLine{Level: LevelDebug}
	s.True(line.MatchesLevel(""))
}

func TestParserTestSuite(t *testing.T) {
	suite.Run(t, new(ParserTestSuite))
}
