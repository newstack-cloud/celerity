package devlogs

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FormatterTestSuite struct {
	suite.Suite
	formatter *Formatter
}

func (s *FormatterTestSuite) SetupTest() {
	s.formatter = &Formatter{UseColor: false}
}

func (s *FormatterTestSuite) Test_basic_message_with_handler() {
	line := LogLine{
		Level:       LevelInfo,
		Message:     "order created",
		HandlerName: "Orders",
	}
	out := s.formatter.Format(line)
	s.Contains(out, "INFO")
	s.Contains(out, "[Orders")
	s.Contains(out, "order created")
}

func (s *FormatterTestSuite) Test_basic_message_without_handler() {
	line := LogLine{
		Level:   LevelWarn,
		Message: "deprecation notice",
	}
	out := s.formatter.Format(line)
	s.Contains(out, "WARN")
	s.Contains(out, "deprecation notice")
	s.NotContains(out, "[")
}

func (s *FormatterTestSuite) Test_timestamp_included() {
	line := LogLine{
		Timestamp: "2026-02-20T10:00:00Z",
		Level:     LevelInfo,
		Message:   "hello",
	}
	out := s.formatter.Format(line)
	s.Contains(out, "2026-02-20T10:00:00Z")
}

func (s *FormatterTestSuite) Test_extra_fields_on_indented_lines() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "order created",
		Extra: map[string]any{
			"orderId":  "ord-123",
			"duration": float64(45),
		},
	}
	out := s.formatter.Format(line)
	s.Contains(out, "order created")
	s.Contains(out, "\n    duration: 45")
	s.Contains(out, "\n    orderId: ord-123")
}

func (s *FormatterTestSuite) Test_extra_fields_sorted_alphabetically() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "test",
		Extra: map[string]any{
			"zebra": "z",
			"alpha": "a",
			"mid":   "m",
		},
	}
	out := s.formatter.Format(line)
	alphaIdx := indexOf(out, "alpha:")
	midIdx := indexOf(out, "mid:")
	zebraIdx := indexOf(out, "zebra:")
	s.Greater(midIdx, alphaIdx)
	s.Greater(zebraIdx, midIdx)
}

func (s *FormatterTestSuite) Test_no_extra_lines_when_empty() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "simple",
	}
	out := s.formatter.Format(line)
	s.NotContains(out, "\n")
}

func (s *FormatterTestSuite) Test_nested_object_in_extra() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "test",
		Extra: map[string]any{
			"nested": map[string]any{"key": "val"},
		},
	}
	out := s.formatter.Format(line)
	s.Contains(out, `nested: {"key":"val"}`)
}

func (s *FormatterTestSuite) Test_system_level_passthrough_no_prefix() {
	raw := "2026-02-21T12:08:36.754Z celerity:core:runtime-entry guard admin — input method=POST"
	line := LogLine{Level: LevelSystem, Message: raw}
	out := s.formatter.Format(line)
	s.Equal(raw, out)
	s.NotContains(out, "INFO")
	s.NotContains(out, "SYSTEM")
}

func (s *FormatterTestSuite) Test_runtime_source_shows_runtime_label() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "HTTP request received",
		Source:  "runtime",
	}
	out := s.formatter.Format(line)
	s.Contains(out, "[runtime")
	s.Contains(out, "HTTP request received")
}

func (s *FormatterTestSuite) Test_runtime_source_with_handler_name_shows_combined_label() {
	line := LogLine{
		Level:       LevelWarn,
		Message:     "guard rejected",
		HandlerName: "Orders",
		Source:      "runtime",
	}
	out := s.formatter.Format(line)
	s.Contains(out, "[runtime - Orders")
}

func (s *FormatterTestSuite) Test_bool_and_nil_in_extra() {
	line := LogLine{
		Level:   LevelInfo,
		Message: "test",
		Extra: map[string]any{
			"active": true,
			"data":   nil,
		},
	}
	out := s.formatter.Format(line)
	s.Contains(out, "active: true")
	s.Contains(out, "data: null")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestFormatterTestSuite(t *testing.T) {
	suite.Run(t, new(FormatterTestSuite))
}
