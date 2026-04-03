package devlogs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogFileWriter demultiplexes parsed log lines into per-handler files
// and a combined log file. All output is plain text (no ANSI).
type LogFileWriter struct {
	logDir    string
	allFile   *os.File
	infraFile *os.File
	handlers  map[string]*os.File
	mu        sync.Mutex
}

// NewLogFileWriter creates a LogFileWriter that writes to {celerityDir}/logs/.
// Opens all.log and infra.log in append mode. Handler files are opened lazily.
func NewLogFileWriter(celerityDir string) (*LogFileWriter, error) {
	logDir := filepath.Join(celerityDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory %s: %w", logDir, err)
	}

	allFile, err := openAppend(filepath.Join(logDir, "all.log"))
	if err != nil {
		return nil, fmt.Errorf("opening all.log: %w", err)
	}

	infraFile, err := openAppend(filepath.Join(logDir, "infra.log"))
	if err != nil {
		allFile.Close()
		return nil, fmt.Errorf("opening infra.log: %w", err)
	}

	return &LogFileWriter{
		logDir:    logDir,
		allFile:   allFile,
		infraFile: infraFile,
		handlers:  make(map[string]*os.File),
	}, nil
}

// Write writes a parsed log line to all.log and the appropriate handler/infra file.
func (w *LogFileWriter) Write(line LogLine) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	formatted := formatForFile(line)
	if _, err := fmt.Fprintln(w.allFile, formatted); err != nil {
		return err
	}

	if line.HandlerName == "" {
		_, err := fmt.Fprintln(w.infraFile, formatted)
		return err
	}

	f, err := w.handlerFile(line.HandlerName)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, formatted)
	return err
}

// Close closes all open file handles.
func (w *LogFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	for _, f := range w.handlers {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := w.infraFile.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := w.allFile.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// LogDir returns the path to the log directory.
func (w *LogFileWriter) LogDir() string {
	return w.logDir
}

// HandlerFiles returns the names of handler-specific log files that have been written.
func (w *LogFileWriter) HandlerFiles() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	names := make([]string, 0, len(w.handlers))
	for name := range w.handlers {
		names = append(names, sanitizeFilename(name)+".log")
	}
	return names
}

// CleanLogDir removes the log directory and its contents.
func CleanLogDir(celerityDir string) error {
	logDir := filepath.Join(celerityDir, "logs")
	return os.RemoveAll(logDir)
}

func (w *LogFileWriter) handlerFile(name string) (*os.File, error) {
	if f, ok := w.handlers[name]; ok {
		return f, nil
	}

	safe := sanitizeFilename(name)
	path := filepath.Join(w.logDir, safe+".log")
	f, err := openAppend(path)
	if err != nil {
		return nil, fmt.Errorf("opening handler log %s: %w", path, err)
	}
	w.handlers[name] = f
	return f, nil
}

func formatForFile(line LogLine) string {
	// System lines (e.g. Node.js debug output) are stored as-is.
	if line.Level == LevelSystem {
		return line.Message
	}

	ts := line.Timestamp
	if ts == "" {
		ts = time.Now().Format(time.RFC3339)
	}
	level := line.Level
	if level == "" {
		level = "INFO"
	}

	label := buildHandlerLabel(line)
	if label != "" {
		return fmt.Sprintf("%s [%s] %-5s %s", ts, label, level, line.Message)
	}
	return fmt.Sprintf("%s %-5s %s", ts, level, line.Message)
}

func sanitizeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result = append(result, '_')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

func openAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}
