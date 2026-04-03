package devlogs

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorBlue   = "\033[34m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
)

// Formatter formats parsed log lines with optional ANSI colors.
type Formatter struct {
	UseColor bool
}

// Format renders a LogLine as a human-readable string.
// Output includes timestamp, level, handler prefix, message, and
// any extra fields on indented lines below the message.
func (f *Formatter) Format(line LogLine) string {
	var b strings.Builder

	f.writeHeader(&b, line)
	f.writeExtra(&b, line.Extra)

	return b.String()
}

func (f *Formatter) writeHeader(b *strings.Builder, line LogLine) {
	// System lines (e.g. Node.js debug output) pass through without a level prefix.
	if line.Level == LevelSystem {
		b.WriteString(line.Message)
		return
	}

	level := f.colorLevel(line.Level)

	if line.Timestamp != "" {
		ts := f.dim(line.Timestamp)
		fmt.Fprintf(b, "%s ", ts)
	}

	handlerLabel := buildHandlerLabel(line)

	if handlerLabel != "" {
		fmt.Fprintf(b, "%s [%s] %s", level, handlerLabel, line.Message)
		return
	}
	fmt.Fprintf(b, "%s %s", level, line.Message)
}

func buildHandlerLabel(line LogLine) string {
	if line.Source == "runtime" {
		if line.HandlerName != "" {
			return "runtime - " + line.HandlerName
		}
		return "runtime"
	}
	return line.HandlerName
}

func (f *Formatter) writeExtra(b *strings.Builder, extra map[string]any) {
	if len(extra) == 0 {
		return
	}

	keys := sortedKeys(extra)
	for _, k := range keys {
		v := formatValue(extra[k])
		line := fmt.Sprintf("    %s: %s", k, v)
		b.WriteString("\n")
		b.WriteString(f.dim(line))
	}
}

func (f *Formatter) dim(s string) string {
	if !f.UseColor {
		return s
	}
	return colorGray + s + colorReset
}

func (f *Formatter) colorLevel(level string) string {
	padded := fmt.Sprintf("%-5s", level)
	if !f.UseColor {
		return padded
	}

	color, ok := levelColors[level]
	if !ok {
		return padded
	}
	return color + padded + colorReset
}

var levelColors = map[string]string{
	LevelDebug: colorGray,
	LevelInfo:  colorBlue,
	LevelWarn:  colorYellow,
	LevelError: colorRed,
}

func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case nil:
		return "null"
	default:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
