package devlogs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type LogFileWriterTestSuite struct {
	suite.Suite
	celerityDir string
}

func (s *LogFileWriterTestSuite) SetupTest() {
	s.celerityDir = filepath.Join(s.T().TempDir(), ".celerity")
}

func (s *LogFileWriterTestSuite) Test_writes_handler_line_to_all_and_handler_file() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp:   "2024-01-01T12:00:00Z",
		Level:       "INFO",
		Message:     "order created",
		HandlerName: "Create-Order-v1",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	allContent := s.readLog("all.log")
	s.Assert().Contains(allContent, "[Create-Order-v1]")
	s.Assert().Contains(allContent, "order created")

	handlerContent := s.readLog("Create-Order-v1.log")
	s.Assert().Contains(handlerContent, "[Create-Order-v1]")
	s.Assert().Contains(handlerContent, "order created")

	// Should NOT be in infra.log
	infraContent := s.readLog("infra.log")
	s.Assert().Empty(strings.TrimSpace(infraContent))
}

func (s *LogFileWriterTestSuite) Test_writes_infra_line_to_all_and_infra_file() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z",
		Level:     "INFO",
		Message:   "server started on :8080",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	allContent := s.readLog("all.log")
	s.Assert().Contains(allContent, "server started")

	infraContent := s.readLog("infra.log")
	s.Assert().Contains(infraContent, "server started")
}

func (s *LogFileWriterTestSuite) Test_multiple_handlers_produce_separate_files() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z", Level: "INFO",
		Message: "hello", HandlerName: "Handler-A",
	})
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:01Z", Level: "WARN",
		Message: "world", HandlerName: "Handler-B",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	aContent := s.readLog("Handler-A.log")
	s.Assert().Contains(aContent, "hello")
	s.Assert().NotContains(aContent, "world")

	bContent := s.readLog("Handler-B.log")
	s.Assert().Contains(bContent, "world")
	s.Assert().NotContains(bContent, "hello")

	// all.log has both
	allContent := s.readLog("all.log")
	s.Assert().Contains(allContent, "hello")
	s.Assert().Contains(allContent, "world")
}

func (s *LogFileWriterTestSuite) Test_handler_file_opened_lazily() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	// Before any handler write, only all.log and infra.log should exist.
	logDir := filepath.Join(s.celerityDir, "logs")
	entries, _ := os.ReadDir(logDir)
	names := fileNames(entries)
	s.Assert().Contains(names, "all.log")
	s.Assert().Contains(names, "infra.log")
	s.Assert().Len(entries, 2, "only all.log and infra.log before first handler write")

	// Write a handler line to trigger lazy file creation.
	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z", Level: "INFO",
		Message: "test", HandlerName: "Lazy-Handler",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	entries, _ = os.ReadDir(logDir)
	names = fileNames(entries)
	s.Assert().Contains(names, "Lazy-Handler.log")
}

func (s *LogFileWriterTestSuite) Test_sanitizes_problematic_characters() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z", Level: "INFO",
		Message: "test", HandlerName: "path/to:handler",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	content := s.readLog("path_to_handler.log")
	s.Assert().Contains(content, "test")
}

func (s *LogFileWriterTestSuite) Test_clean_log_dir_removes_contents() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z", Level: "INFO",
		Message: "test", HandlerName: "Handler-A",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	logDir := filepath.Join(s.celerityDir, "logs")
	entries, _ := os.ReadDir(logDir)
	s.Assert().NotEmpty(entries)

	err = CleanLogDir(s.celerityDir)
	s.Require().NoError(err)

	_, err = os.Stat(logDir)
	s.Assert().True(os.IsNotExist(err), "log directory should be removed")
}

func (s *LogFileWriterTestSuite) Test_handler_files_returns_written_handler_names() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	_ = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:00Z", Level: "INFO",
		Message: "a", HandlerName: "Alpha",
	})
	_ = w.Write(LogLine{
		Timestamp: "2024-01-01T12:00:01Z", Level: "INFO",
		Message: "b", HandlerName: "Beta",
	})

	files := w.HandlerFiles()
	s.Assert().Len(files, 2)
	s.Assert().Contains(files, "Alpha.log")
	s.Assert().Contains(files, "Beta.log")

	s.Require().NoError(w.Close())
}

func (s *LogFileWriterTestSuite) Test_system_level_stored_as_raw_message() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	raw := "2026-02-21T12:08:36.754Z celerity:core:runtime-entry guard admin — input"
	err = w.Write(LogLine{Level: LevelSystem, Message: raw})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	content := s.readLog("all.log")
	s.Assert().Contains(content, raw)
	s.Assert().NotContains(content, "SYSTEM")
}

func (s *LogFileWriterTestSuite) Test_runtime_source_uses_runtime_label() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp: "2026-02-19T14:23:01Z",
		Level:     LevelInfo,
		Message:   "HTTP request received",
		Source:    "runtime",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	content := s.readLog("all.log")
	s.Assert().Contains(content, "[runtime]")
	infraContent := s.readLog("infra.log")
	s.Assert().Contains(infraContent, "[runtime]")
}

func (s *LogFileWriterTestSuite) Test_runtime_source_with_handler_uses_combined_label() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Timestamp:   "2026-02-19T14:23:01Z",
		Level:       LevelWarn,
		Message:     "guard rejected",
		HandlerName: "Orders",
		Source:      "runtime",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	content := s.readLog("all.log")
	s.Assert().Contains(content, "[runtime - Orders]")
	handlerContent := s.readLog("Orders.log")
	s.Assert().Contains(handlerContent, "[runtime - Orders]")
}

func (s *LogFileWriterTestSuite) Test_defaults_empty_level_and_timestamp() {
	w, err := NewLogFileWriter(s.celerityDir)
	s.Require().NoError(err)

	err = w.Write(LogLine{
		Message: "bare line",
	})
	s.Require().NoError(err)
	s.Require().NoError(w.Close())

	content := s.readLog("all.log")
	s.Assert().Contains(content, "INFO")
	s.Assert().Contains(content, "bare line")
}

func (s *LogFileWriterTestSuite) readLog(name string) string {
	data, err := os.ReadFile(filepath.Join(s.celerityDir, "logs", name))
	s.Require().NoError(err, "failed to read %s", name)
	return string(data)
}

func fileNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}

func TestLogFileWriterTestSuite(t *testing.T) {
	suite.Run(t, new(LogFileWriterTestSuite))
}
