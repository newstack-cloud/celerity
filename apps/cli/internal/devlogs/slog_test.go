package devlogs

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// The lines asserted here were produced by the Celerity Go SDK rather than
// written to look like it, since what the parser has to agree with is what
// log/slog's JSON handler actually emits: the fields named time, level and
// msg, and a level in upper case.
type SlogSuite struct {
	suite.Suite
	parser *LogParser
}

func TestSlogSuite(t *testing.T) { suite.Run(t, new(SlogSuite)) }

func (s *SlogSuite) SetupTest() {
	s.parser = NewLogParser("os-only.debian13")
}

func (s *SlogSuite) Test_a_record_keeps_its_time_level_and_message() {
	line := s.parser.Parse(
		`{"time":"2026-10-09T02:43:48.431277+01:00","level":"INFO","msg":"order created",` +
			`"orderId":"o-1","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}`)

	s.Assert().Equal("2026-10-09T02:43:48.431277+01:00", line.Timestamp)
	s.Assert().Equal(LevelInfo, line.Level)
	s.Assert().Equal("order created", line.Message)
}

// Without this the whole line is printed as the message and every record
// reads at info, which is what a Go session got before the parser existed.
func (s *SlogSuite) Test_every_level_is_recognised() {
	for raw, want := range map[string]string{
		`{"time":"t","level":"DEBUG","msg":"m"}`: LevelDebug,
		`{"time":"t","level":"INFO","msg":"m"}`:  LevelInfo,
		`{"time":"t","level":"WARN","msg":"m"}`:  LevelWarn,
		`{"time":"t","level":"ERROR","msg":"m"}`: LevelError,
	} {
		s.Assert().Equal(want, s.parser.Parse(raw).Level, raw)
	}
}

// Bound by the SDK before a handler has added anything, so they are the
// correlation a reader scanning a session is looking for rather than
// attributes among the application's own.
func (s *SlogSuite) Test_the_handler_and_the_event_are_lifted_out_of_the_attributes() {
	line := s.parser.Parse(
		`{"time":"t","level":"INFO","msg":"dispatched","handler":"createOrder",` +
			`"tag":"http","eventId":"ev-1"}`)

	s.Assert().Equal("createOrder", line.HandlerName)
	s.Assert().Equal("ev-1", line.RequestID)
	s.Assert().Equal("http", line.Extra["tag"], "the rest stay attributes")
	s.Assert().NotContains(line.Extra, "handler")
	s.Assert().NotContains(line.Extra, "msg")
}

// The Rust core runtime also spells its levels in upper case, and its records
// are the ones with a fields object. They have to keep going to the tracing
// parser, which names the span a line came from.
func (s *SlogSuite) Test_a_core_runtime_record_is_still_read_as_one() {
	line := s.parser.Parse(
		`{"timestamp":"t","level":"INFO","fields":{"message":"listening"},"target":"celerity"}`)

	s.Assert().Equal("runtime", line.Source)
	s.Assert().Equal("listening", line.Message)
}

func (s *SlogSuite) Test_json_that_is_not_a_record_falls_through() {
	for _, raw := range []string{
		`{"level":"INFO"}`,
		`{"msg":"no level"}`,
		`{"level":"TRACE","msg":"not one of slog's"}`,
		`{"level":42,"msg":"m"}`,
	} {
		_, ok := parseSlogJSON(raw)
		s.Assert().False(ok, raw)
	}
}

// A compiled language the CLI has no parser for still reaches the reader.
func (s *SlogSuite) Test_a_line_that_is_not_json_is_passed_through() {
	line := s.parser.Parse("plain text from another sdk")

	s.Assert().Equal("plain text from another sdk", line.Message)
	s.Assert().Equal(LevelInfo, line.Level)
}
