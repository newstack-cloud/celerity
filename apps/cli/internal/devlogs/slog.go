package devlogs

import (
	"encoding/json"
	"strings"
)

// The Celerity Go SDK writes through log/slog, whose JSON handler names the
// three record fields time, level and msg, and renders a level in upper case.
// Neither matches structlog, which writes timestamp and lower-case names, so
// reusing the Python parser for it would leave every record undated and at
// the default level.

// A dispatch arrives already carrying these, bound by the SDK before the
// application has added anything of its own.
const (
	slogHandlerKey = "handler"
	slogEventIDKey = "eventId"
)

var slogKnownKeys = map[string]bool{
	"time": true, "level": true, "msg": true,
	slogHandlerKey: true, slogEventIDKey: true,
}

// slog's own set, and only these four, so a line that merely carries a level is
// not taken for an slog record. The Rust core runtime's tracing JSON also spells
// its levels in upper case, and the two are separated by that one carrying a
// fields object, which parseTracingJSON checks for first.
var slogLevels = map[string]bool{
	LevelDebug: true, LevelInfo: true,
	LevelWarn: true, LevelError: true,
}

func parseSlogJSON(raw string) (LogLine, bool) {
	if len(raw) == 0 || raw[0] != '{' {
		return LogLine{}, false
	}

	var all map[string]any
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		return LogLine{}, false
	}

	level, hasLevel := all["level"].(string)
	if !hasLevel || !slogLevels[strings.ToUpper(level)] {
		return LogLine{}, false
	}
	// A record has a message, and demanding one is what keeps an arbitrary
	// JSON object that happens to carry a level from being read as a record.
	if _, hasMessage := all["msg"].(string); !hasMessage {
		return LogLine{}, false
	}

	return LogLine{
		Timestamp:   stringFromMap(all, "time"),
		Level:       strings.ToUpper(level),
		Message:     stringFromMap(all, "msg"),
		HandlerName: stringFromMap(all, slogHandlerKey),
		// The SDK's own correlation id for one dispatch, which is what the
		// other two SDKs call a request id.
		RequestID: stringFromMap(all, slogEventIDKey),
		RawLine:   raw,
		Extra:     collectExtra(all, slogKnownKeys),
	}, true
}
