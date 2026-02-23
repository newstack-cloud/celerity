package bridge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

type ScheduleTriggerSuite struct {
	suite.Suite
	rdb    *redis.Client
	logger *zap.Logger
}

func (s *ScheduleTriggerSuite) SetupTest() {
	s.rdb = newTestRedisClient(s.T())
	s.logger = newTestLogger(s.T())
}

func (s *ScheduleTriggerSuite) TearDownTest() {
	_ = s.rdb.Close()
}

func (s *ScheduleTriggerSuite) newTrigger() *ScheduleTrigger {
	return NewScheduleTrigger(s.rdb, s.logger)
}

func (s *ScheduleTriggerSuite) parseBody(raw string) map[string]any {
	var body map[string]any
	err := json.Unmarshal([]byte(raw), &body)
	s.Require().NoError(err, "failed to parse schedule event body JSON")
	return body
}

func (s *ScheduleTriggerSuite) Test_writeScheduleEvent_writes_correct_fields() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "sched-1",
		Schedule: "rate(5 minutes)",
		Stream:   stream,
		Input:    map[string]any{"key": "value"},
	}

	st := s.newTrigger()
	st.writeScheduleEvent(context.Background(), entry, s.logger)

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)

	s.Assert().Equal("0", msgs[0].Values["message_type"])

	body := s.parseBody(msgs[0].Values["body"].(string))
	s.Assert().Equal("sched-1", body["scheduleId"])
	s.Assert().NotEmpty(body["scheduledTime"])

	input, ok := body["input"].(map[string]any)
	s.Require().True(ok, "input should be a map")
	s.Assert().Equal("value", input["key"])
}

func (s *ScheduleTriggerSuite) Test_writeScheduleEvent_with_nil_input() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "sched-nil",
		Schedule: "rate(1 minutes)",
		Stream:   stream,
		Input:    nil,
	}

	st := s.newTrigger()
	st.writeScheduleEvent(context.Background(), entry, s.logger)

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)

	body := s.parseBody(msgs[0].Values["body"].(string))
	s.Assert().Nil(body["input"])
}

func (s *ScheduleTriggerSuite) Test_writeScheduleEvent_with_string_input() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "sched-str",
		Schedule: "rate(1 minutes)",
		Stream:   stream,
		Input:    "hello",
	}

	st := s.newTrigger()
	st.writeScheduleEvent(context.Background(), entry, s.logger)

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)

	body := s.parseBody(msgs[0].Values["body"].(string))
	s.Assert().Equal("hello", body["input"])
}

func (s *ScheduleTriggerSuite) Test_writeScheduleEvent_multiple_invocations() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "sched-multi",
		Schedule: "rate(5 minutes)",
		Stream:   stream,
		Input:    nil,
	}

	st := s.newTrigger()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		st.writeScheduleEvent(ctx, entry, s.logger)
	}

	msgs := readStreamMessages(s.T(), s.rdb, stream, 3, 5*time.Second)
	s.Require().Len(msgs, 3)

	// All should have distinct stream IDs.
	ids := make(map[string]bool)
	for _, m := range msgs {
		s.Assert().False(ids[m.ID], "duplicate stream ID: %s", m.ID)
		ids[m.ID] = true

		body := s.parseBody(m.Values["body"].(string))
		s.Assert().Equal("sched-multi", body["scheduleId"])
	}
}

func (s *ScheduleTriggerSuite) Test_writeScheduleEvent_scheduledTime_is_valid_RFC3339() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "sched-time",
		Schedule: "rate(1 minutes)",
		Stream:   stream,
		Input:    nil,
	}

	before := time.Now().UTC()
	st := s.newTrigger()
	st.writeScheduleEvent(context.Background(), entry, s.logger)
	after := time.Now().UTC()

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)

	body := s.parseBody(msgs[0].Values["body"].(string))
	scheduledTimeStr, ok := body["scheduledTime"].(string)
	s.Require().True(ok, "scheduledTime should be a string")

	parsed, err := time.Parse(time.RFC3339, scheduledTimeStr)
	s.Require().NoError(err, "scheduledTime should be valid RFC3339")
	s.Assert().False(parsed.Before(before.Add(-1*time.Second)), "scheduledTime too early")
	s.Assert().False(parsed.After(after.Add(1*time.Second)), "scheduledTime too late")
}

// --- runSchedule dispatch tests ---

func (s *ScheduleTriggerSuite) Test_runSchedule_dispatches_rate_expression() {
	// runRateSchedule blocks on a ticker; cancel quickly to verify
	// it enters the rate path without error.
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "rate-dispatch",
		Schedule: "rate(1 minutes)",
		Stream:   stream,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	// runSchedule blocks until ctx is cancelled; it should not panic or log an error.
	st.runSchedule(ctx, entry)
}

func (s *ScheduleTriggerSuite) Test_runSchedule_dispatches_cron_expression() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "cron-dispatch",
		Schedule: "cron(* * * * *)",
		Stream:   stream,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	st.runSchedule(ctx, entry)
}

func (s *ScheduleTriggerSuite) Test_runSchedule_unrecognized_expression_returns_immediately() {
	entry := config.ScheduleEntry{
		ID:       "bad-expr",
		Schedule: "every tuesday",
		Stream:   uniqueStream(s.T(), "sched"),
	}

	st := s.newTrigger()
	done := make(chan struct{})
	go func() {
		defer close(done)
		st.runSchedule(context.Background(), entry)
	}()

	select {
	case <-done:
		// returned immediately — correct
	case <-time.After(2 * time.Second):
		s.Fail("runSchedule with unrecognized expression should return immediately")
	}
}

// --- runRateSchedule unit variations ---

func (s *ScheduleTriggerSuite) Test_runRateSchedule_hour_unit() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "rate-hour",
		Schedule: "rate(1 hour)",
		Stream:   stream,
	}
	matches := ratePattern.FindStringSubmatch(entry.Schedule)
	s.Require().NotNil(matches)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	st.runRateSchedule(ctx, entry, matches, s.logger)
}

func (s *ScheduleTriggerSuite) Test_runRateSchedule_day_unit() {
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "rate-day",
		Schedule: "rate(1 days)",
		Stream:   stream,
	}
	matches := ratePattern.FindStringSubmatch(entry.Schedule)
	s.Require().NotNil(matches)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	st.runRateSchedule(ctx, entry, matches, s.logger)
}

// --- runCronSchedule variations ---

func (s *ScheduleTriggerSuite) Test_runCronSchedule_aws_format_with_year_field() {
	// AWS EventBridge 6-field cron: min hour dom month dow year
	// The year field should be stripped.
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "cron-aws",
		Schedule: "cron(0 12 * * ? 2026)",
		Stream:   stream,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	st.runCronSchedule(ctx, entry, s.logger)
}

func (s *ScheduleTriggerSuite) Test_runCronSchedule_question_mark_replaced() {
	// AWS uses "?" for no specific value; robfig/cron expects "*".
	stream := uniqueStream(s.T(), "sched")
	entry := config.ScheduleEntry{
		ID:       "cron-qmark",
		Schedule: "cron(0 12 ? * MON)",
		Stream:   stream,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	st := s.newTrigger()
	st.runCronSchedule(ctx, entry, s.logger)
}

func (s *ScheduleTriggerSuite) Test_runCronSchedule_invalid_expression_returns_immediately() {
	entry := config.ScheduleEntry{
		ID:       "cron-bad",
		Schedule: "cron(not a cron)",
		Stream:   uniqueStream(s.T(), "sched"),
	}

	st := s.newTrigger()
	done := make(chan struct{})
	go func() {
		defer close(done)
		st.runCronSchedule(context.Background(), entry, s.logger)
	}()

	select {
	case <-done:
		// returned immediately — correct
	case <-time.After(2 * time.Second):
		s.Fail("runCronSchedule with invalid expression should return immediately")
	}
}

// --- Start test ---

func (s *ScheduleTriggerSuite) Test_Start_spawns_goroutines_and_stops_on_cancel() {
	entries := []config.ScheduleEntry{
		{ID: "s1", Schedule: "rate(1 minutes)", Stream: uniqueStream(s.T(), "sched1")},
		{ID: "s2", Schedule: "cron(* * * * *)", Stream: uniqueStream(s.T(), "sched2")},
	}

	ctx, cancel := context.WithCancel(context.Background())
	st := s.newTrigger()

	done := make(chan struct{})
	go func() {
		defer close(done)
		st.Start(ctx, entries)
	}()

	// Start spawns goroutines and returns immediately — give them a moment to start.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		s.Fail("Start should return after context cancel")
	}
}

func TestScheduleTriggerSuite(t *testing.T) {
	suite.Run(t, new(ScheduleTriggerSuite))
}
