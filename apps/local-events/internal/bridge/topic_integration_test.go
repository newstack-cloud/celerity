package bridge

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"

	"github.com/newstack-cloud/celerity/apps/local-events/internal/config"
)

type TopicBridgeSuite struct {
	suite.Suite
	rdb    *redis.Client
	logger *zap.Logger
}

func (s *TopicBridgeSuite) SetupTest() {
	s.rdb = newTestRedisClient(s.T())
	s.logger = newTestLogger(s.T())
}

func (s *TopicBridgeSuite) TearDownTest() {
	_ = s.rdb.Close()
}

// startBridge launches the TopicBridge in a goroutine and returns a function
// that cancels the context and waits for the goroutine to finish.
func (s *TopicBridgeSuite) startBridge(
	source *config.TopicBridgeSource,
	targets []config.TopicBridgeTarget,
) (cancel func()) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tb := NewTopicBridge(s.rdb, s.logger)
		tb.Start(ctx, source, targets)
	}()
	// Allow time for the pub/sub subscription to be established.
	time.Sleep(100 * time.Millisecond)
	return func() {
		cancelCtx()
		wg.Wait()
	}
}

func (s *TopicBridgeSuite) Test_single_target_receives_published_message() {
	channel := uniqueStream(s.T(), "chan")
	stream := uniqueStream(s.T(), "target")

	stop := s.startBridge(
		&config.TopicBridgeSource{Channel: channel},
		[]config.TopicBridgeTarget{{Stream: stream}},
	)
	defer stop()

	err := s.rdb.Publish(context.Background(), channel, "hello world").Err()
	s.Require().NoError(err)

	msgs := readStreamMessages(s.T(), s.rdb, stream, 1, 5*time.Second)
	s.Require().Len(msgs, 1)

	s.Assert().Equal("hello world", msgs[0].Values["body"])
	s.Assert().Equal("0", msgs[0].Values["message_type"])

	ts, err := strconv.ParseInt(msgs[0].Values["timestamp"].(string), 10, 64)
	s.Require().NoError(err)
	s.Assert().InDelta(time.Now().Unix(), ts, 5)
}

func (s *TopicBridgeSuite) Test_fan_out_to_multiple_targets() {
	channel := uniqueStream(s.T(), "chan")
	streams := []config.TopicBridgeTarget{
		{Stream: uniqueStream(s.T(), "t1")},
		{Stream: uniqueStream(s.T(), "t2")},
		{Stream: uniqueStream(s.T(), "t3")},
	}

	stop := s.startBridge(
		&config.TopicBridgeSource{Channel: channel},
		streams,
	)
	defer stop()

	err := s.rdb.Publish(context.Background(), channel, "fan-out-payload").Err()
	s.Require().NoError(err)

	for _, target := range streams {
		msgs := readStreamMessages(s.T(), s.rdb, target.Stream, 1, 5*time.Second)
		s.Require().Len(msgs, 1, "expected 1 message on stream %s", target.Stream)
		s.Assert().Equal("fan-out-payload", msgs[0].Values["body"])
	}
}

func (s *TopicBridgeSuite) Test_multiple_messages_arrive_in_order() {
	channel := uniqueStream(s.T(), "chan")
	stream := uniqueStream(s.T(), "target")

	stop := s.startBridge(
		&config.TopicBridgeSource{Channel: channel},
		[]config.TopicBridgeTarget{{Stream: stream}},
	)
	defer stop()

	for i := 0; i < 5; i++ {
		err := s.rdb.Publish(context.Background(), channel, "msg-"+strconv.Itoa(i)).Err()
		s.Require().NoError(err)
		time.Sleep(10 * time.Millisecond)
	}

	msgs := readStreamMessages(s.T(), s.rdb, stream, 5, 5*time.Second)
	s.Require().Len(msgs, 5)
	for i, msg := range msgs {
		s.Assert().Equal("msg-"+strconv.Itoa(i), msg.Values["body"])
	}
}

func (s *TopicBridgeSuite) Test_no_messages_before_publish() {
	channel := uniqueStream(s.T(), "chan")
	stream := uniqueStream(s.T(), "target")

	stop := s.startBridge(
		&config.TopicBridgeSource{Channel: channel},
		[]config.TopicBridgeTarget{{Stream: stream}},
	)
	defer stop()

	// Wait a moment, then verify the stream is empty.
	time.Sleep(200 * time.Millisecond)
	ctx := context.Background()
	length, err := s.rdb.XLen(ctx, stream).Result()
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), length)
}

func (s *TopicBridgeSuite) Test_bridge_stops_on_context_cancel() {
	channel := uniqueStream(s.T(), "chan")
	stream := uniqueStream(s.T(), "target")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		tb := NewTopicBridge(s.rdb, s.logger)
		tb.Start(ctx, &config.TopicBridgeSource{Channel: channel}, []config.TopicBridgeTarget{{Stream: stream}})
	}()

	// Give the bridge a moment to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		s.Fail("bridge goroutine did not stop within 2s of context cancel")
	}
}

func TestTopicBridgeSuite(t *testing.T) {
	suite.Run(t, new(TopicBridgeSuite))
}
