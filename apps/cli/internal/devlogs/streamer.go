package devlogs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
)

const maxScanTokenSize = 1024 * 1024

// StreamOptions configures log streaming with filtering.
type StreamOptions struct {
	Follow        bool
	Tail          string
	Since         string
	HandlerFilter string
	LevelFilter   string
}

// SyncResult summarises a SyncToFiles operation.
type SyncResult struct {
	LogDir       string
	TotalLines   int
	HandlerFiles []string
}

// Streamer reads container logs from the Docker API, parses them,
// applies filters, and writes formatted output.
type Streamer struct {
	docker    docker.RuntimeContainerManager
	formatter *Formatter
	parser    *LogParser
	// FileWriter optionally tees parsed lines to per-handler log files.
	FileWriter *LogFileWriter
}

// NewStreamer creates a log streamer configured for the given application runtime.
func NewStreamer(
	dockerMgr docker.RuntimeContainerManager,
	runtime string,
	useColor bool,
) *Streamer {
	return &Streamer{
		docker:    dockerMgr,
		formatter: &Formatter{UseColor: useColor},
		parser:    NewLogParser(runtime),
	}
}

// Stream reads and formats container logs to the given writer.
// Parses each line independently (JSON or plain-text passthrough).
// Blocks until context cancellation or stream end.
func (s *Streamer) Stream(
	ctx context.Context,
	containerID string,
	opts StreamOptions,
	writer io.Writer,
) error {
	reader, err := s.openLogStream(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		s.handleSingleLine(scanner.Text(), opts, writer)
	}

	return scanner.Err()
}

func (s *Streamer) handleSingleLine(
	raw string,
	opts StreamOptions,
	writer io.Writer,
) {
	line := s.parser.Parse(raw)

	if s.FileWriter != nil {
		_ = s.FileWriter.Write(line)
	}

	if !line.MatchesHandler(opts.HandlerFilter) {
		return
	}
	if !line.MatchesLevel(opts.LevelFilter) {
		return
	}

	fmt.Fprintln(writer, s.formatter.Format(line))
}

// SyncToFiles reads all available container logs and writes them to log files
// without any stdout output. FileWriter must be set before calling.
func (s *Streamer) SyncToFiles(
	ctx context.Context,
	containerID string,
	opts StreamOptions,
) (*SyncResult, error) {
	if s.FileWriter == nil {
		return nil, fmt.Errorf("FileWriter must be set for SyncToFiles")
	}

	syncOpts := opts
	syncOpts.Follow = false
	if syncOpts.Tail == "" {
		syncOpts.Tail = "all"
	}

	reader, err := s.openLogStream(ctx, containerID, syncOpts)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	totalLines := 0
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := s.parser.Parse(scanner.Text())
		_ = s.FileWriter.Write(line)
		totalLines++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	handlerFiles := s.FileWriter.HandlerFiles()
	sort.Strings(handlerFiles)

	return &SyncResult{
		LogDir:       s.FileWriter.LogDir(),
		TotalLines:   totalLines,
		HandlerFiles: handlerFiles,
	}, nil
}

func (s *Streamer) openLogStream(
	ctx context.Context,
	containerID string,
	opts StreamOptions,
) (io.ReadCloser, error) {
	reader, err := s.docker.StreamLogsWithOptions(ctx, containerID, docker.LogStreamOptions{
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Timestamps: false,
	})
	if err != nil {
		return nil, fmt.Errorf("opening log stream: %w", err)
	}
	return reader, nil
}
