package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

var ratePattern = regexp.MustCompile(`^rate\((\d+)\s+(minutes?|hours?|days?)\)$`)

var rateUnits = map[string]time.Duration{
	"minute": time.Minute,
	"hour":   time.Hour,
	"day":    24 * time.Hour,
}

// ScheduleTrigger manages schedule evaluation and writes trigger messages
// to Valkey streams.
type ScheduleTrigger struct {
	rdb    *redis.Client
	logger *zap.Logger
}

// NewScheduleTrigger creates a new ScheduleTrigger.
func NewScheduleTrigger(rdb *redis.Client, logger *zap.Logger) *ScheduleTrigger {
	return &ScheduleTrigger{rdb: rdb, logger: logger}
}

// Start spawns goroutines for each schedule entry. It blocks until ctx is cancelled.
func (st *ScheduleTrigger) Start(ctx context.Context, schedules []config.ScheduleEntry) {
	for _, entry := range schedules {
		go st.runSchedule(ctx, entry)
	}
}

func (st *ScheduleTrigger) runSchedule(ctx context.Context, entry config.ScheduleEntry) {
	logger := st.logger.With(
		zap.String("schedule_id", entry.ID),
		zap.String("schedule", entry.Schedule),
		zap.String("stream", entry.Stream),
	)

	if matches := ratePattern.FindStringSubmatch(entry.Schedule); matches != nil {
		st.runRateSchedule(ctx, entry, matches, logger)
		return
	}

	if strings.HasPrefix(entry.Schedule, "cron(") && strings.HasSuffix(entry.Schedule, ")") {
		st.runCronSchedule(ctx, entry, logger)
		return
	}

	logger.Error("unrecognised schedule expression, skipping")
}

func (st *ScheduleTrigger) runRateSchedule(
	ctx context.Context,
	entry config.ScheduleEntry,
	matches []string,
	logger *zap.Logger,
) {
	value, _ := strconv.Atoi(matches[1])
	unit := strings.TrimSuffix(matches[2], "s")

	multiplier, ok := rateUnits[unit]
	if !ok {
		logger.Error("unsupported rate unit", zap.String("unit", unit))
		return
	}
	interval := time.Duration(value) * multiplier

	logger.Info("starting rate schedule", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("schedule stopped")
			return
		case <-ticker.C:
			st.writeScheduleEvent(ctx, entry, logger)
		}
	}
}

func (st *ScheduleTrigger) runCronSchedule(
	ctx context.Context,
	entry config.ScheduleEntry,
	logger *zap.Logger,
) {
	inner := entry.Schedule[5 : len(entry.Schedule)-1]
	// AWS EventBridge uses "?" for "no specific value", but robfig/cron expects "*".
	inner = strings.ReplaceAll(inner, "?", "*")

	// AWS EventBridge cron format: minutes hours day-of-month month day-of-week year
	// The robfig/cron library uses standard 5-field cron (minute hour dom month dow)
	// or 6-field with seconds. We strip the year field if present (6 fields → 5 fields for cron).
	fields := strings.Fields(inner)
	if len(fields) == 6 {
		// Drop the year field (last field) — not supported by standard cron libs.
		fields = fields[:5]
	}
	cronExpr := strings.Join(fields, " ")

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		logger.Error("invalid cron expression, skipping", zap.Error(err))
		return
	}

	logger.Info("starting cron schedule", zap.String("cron", cronExpr))

	now := time.Now()
	for {
		next := schedule.Next(now)
		wait := time.Until(next)

		select {
		case <-ctx.Done():
			logger.Info("schedule stopped")
			return
		case <-time.After(wait):
			now = time.Now()
			st.writeScheduleEvent(ctx, entry, logger)
		}
	}
}

func (st *ScheduleTrigger) writeScheduleEvent(
	ctx context.Context,
	entry config.ScheduleEntry,
	logger *zap.Logger,
) {
	now := time.Now()
	body := map[string]any{
		"scheduleId":    entry.ID,
		"scheduledTime": now.UTC().Format(time.RFC3339),
		"input":         entry.Input,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		logger.Error("failed to marshal schedule event body", zap.Error(err))
		return
	}

	err = st.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: entry.Stream,
		Values: map[string]any{
			"body":         string(bodyJSON),
			"timestamp":    fmt.Sprintf("%d", now.Unix()),
			"message_type": "0",
		},
	}).Err()
	if err != nil {
		logger.Error("failed to write schedule event to stream", zap.Error(err))
		return
	}

	logger.Debug("schedule event written", zap.String("stream", entry.Stream))
}
