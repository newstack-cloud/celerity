package devrun

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/newstack-cloud/celerity/apps/cli/internal/blueprint"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devlogs"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devstate"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// Output handles all formatted stdout for the dev commands.
type Output struct {
	writer  io.Writer
	isColor bool
}

// NewOutput creates a new Output writer.
func NewOutput(writer io.Writer, isColor bool) *Output {
	return &Output{writer: writer, isColor: isColor}
}

// PrintStep prints a completed startup step with a green checkmark.
func (o *Output) PrintStep(message string) {
	icon := o.color(colorGreen, "✓")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// PrintError prints a failed step with a red X and error detail.
func (o *Output) PrintError(message string, err error) {
	icon := o.color(colorRed, "✗")
	fmt.Fprintf(o.writer, "  %s %s: %v\n", icon, message, err)
}

// PrintWarning prints a warning with a yellow exclamation mark.
func (o *Output) PrintWarning(message string, err error) {
	icon := o.color(colorYellow, "!")
	fmt.Fprintf(o.writer, "  %s %s: %v\n", icon, message, err)
}

// PrintProgress prints an in-progress message for feedback during slow operations.
func (o *Output) PrintProgress(message string) {
	icon := o.color(colorGray, "…")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// Writer returns the underlying writer for piping external command output.
func (o *Output) Writer() io.Writer {
	return o.writer
}

// PrintInfo prints an informational message.
func (o *Output) PrintInfo(message string) {
	icon := o.color(colorBlue, "●")
	fmt.Fprintf(o.writer, "  %s %s\n", icon, message)
}

// PrintStartupSummary prints the URL and handler table after a successful startup.
func (o *Output) PrintStartupSummary(port string, handlers []blueprint.HandlerInfo) {
	fmt.Fprintln(o.writer)
	url := o.color(colorBold, fmt.Sprintf("http://localhost:%s", port))
	fmt.Fprintf(o.writer, "  App will shortly be listening on %s\n", url)

	if len(handlers) > 0 {
		fmt.Fprintln(o.writer)
		fmt.Fprintln(o.writer, "  Handlers:")
		for _, h := range handlers {
			method := padRight(h.Method, 6)
			path := padRight(h.Path, 20)
			fmt.Fprintf(o.writer, "    %s %s %s\n", method, path, h.HandlerName)
		}
	}

	fmt.Fprintln(o.writer)
}

// PrintStreamingNotice prints the streaming notice for foreground mode.
func (o *Output) PrintStreamingNotice() {
	notice := o.color(colorGray, "Streaming logs... (Ctrl+C to stop)")
	fmt.Fprintf(o.writer, "  %s\n\n", notice)
}

// PrintDetachedNotice prints the background mode notice.
func (o *Output) PrintDetachedNotice() {
	fmt.Fprintln(o.writer, "  Dev environment running in background.")
	fmt.Fprintf(o.writer, "  Use %s to stream logs.\n", o.color(colorBold, "celerity dev logs"))
	fmt.Fprintf(o.writer, "  Use %s to tear down.\n", o.color(colorBold, "celerity dev stop"))
	fmt.Fprintln(o.writer)
}

// PrintShutdownStarting prints the shutdown notice.
func (o *Output) PrintShutdownStarting() {
	fmt.Fprintln(o.writer)
	o.PrintInfo("Stopping dev environment...")
}

// PrintShutdownComplete prints the shutdown completion message.
func (o *Output) PrintShutdownComplete() {
	o.PrintStep("Dev environment stopped")
}

// PrintLogLine formats and prints a parsed log line.
func (o *Output) PrintLogLine(line devlogs.LogLine) {
	f := &devlogs.Formatter{UseColor: o.isColor}
	fmt.Fprintln(o.writer, f.Format(line))
}

// PrintStatus renders the status table for `dev status`.
func (o *Output) PrintStatus(state *devstate.DevState, isRunning bool) {
	status := "Running"
	if !isRunning {
		status = o.color(colorYellow, "Stale (process not running)")
	}

	elapsed := time.Since(state.StartedAt).Truncate(time.Second)
	mode := "foreground"
	if state.Detached {
		mode = "detached"
	}
	if state.PID > 0 {
		mode = fmt.Sprintf("foreground (PID %d)", state.PID)
	}

	fmt.Fprintln(o.writer, "Dev Environment Status")
	fmt.Fprintf(o.writer, "  State:     %s\n", status)
	fmt.Fprintf(o.writer, "  Started:   %s (%s ago)\n", state.StartedAt.Format(time.RFC3339), elapsed)
	fmt.Fprintf(o.writer, "  Container: %s (%s)\n", state.ContainerName, shortID(state.ContainerID))
	fmt.Fprintf(o.writer, "  Image:     %s\n", state.Image)
	fmt.Fprintf(o.writer, "  URL:       http://localhost:%s\n", state.HostPort)
	fmt.Fprintf(o.writer, "  Mode:      %s\n", mode)

	if len(state.Handlers) > 0 {
		fmt.Fprintln(o.writer)
		fmt.Fprintln(o.writer, "  Handlers:")
		for _, h := range state.Handlers {
			method := padRight(h.Method, 6)
			path := padRight(h.Path, 20)
			fmt.Fprintf(o.writer, "    %s %s %s\n", method, path, h.Name)
		}
	}
}

// PrintNoEnvironment prints a message when no dev environment is running.
func (o *Output) PrintNoEnvironment() {
	fmt.Fprintln(o.writer, "No dev environment running.")
}

// PrintTestHeader prints the test run header with the suites being executed.
func (o *Output) PrintTestHeader(suites []string) {
	label := strings.Join(suites, ", ")
	fmt.Fprintf(o.writer, "\n  Running tests (%s)...\n\n", label)
}

// PrintTestPassed prints the success message after all tests pass.
func (o *Output) PrintTestPassed() {
	icon := o.color(colorGreen, "✓")
	fmt.Fprintf(o.writer, "\n  %s %s\n", icon, o.color(colorGreen, "Tests passed"))
}

// PrintTestFailed prints the failure message with the exit code.
func (o *Output) PrintTestFailed(exitCode int) {
	icon := o.color(colorRed, "✗")
	fmt.Fprintf(o.writer, "\n  %s %s (exit code %d)\n", icon, o.color(colorRed, "Tests failed"), exitCode)
}

// PrintHealthWaiting prints a progress message while waiting for the app.
func (o *Output) PrintHealthWaiting() {
	o.PrintProgress("Waiting for app to be ready...")
}

// PrintHealthReady prints a success message when the app is healthy.
func (o *Output) PrintHealthReady(elapsed time.Duration) {
	o.PrintStep(fmt.Sprintf("App ready (%s)", elapsed.Truncate(time.Millisecond)))
}

func (o *Output) color(code string, text string) string {
	if !o.isColor {
		return text
	}
	return code + text + colorReset
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
