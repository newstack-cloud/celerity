package provider

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type CalculateRetryWaitTimeMSTestSuite struct {
	suite.Suite
}

func (s *CalculateRetryWaitTimeMSTestSuite) Test_calculates_wait_time_in_ms() {
	retryPolicy := &RetryPolicy{
		FirstRetryDelay: 2,
		BackoffFactor:   1.5,
		MaxDelay:        14,
		Jitter:          false,
	}

	// First retry attempt should be 2 seconds using interval.
	waitTime1 := CalculateRetryWaitTimeMS(retryPolicy, 1)
	s.Assert().Equal(2000, waitTime1)

	// Second retry attempt should be 3 seconds using
	// first_retry_delay * backoff_factor^(retry_attempt_number-1).
	waitTime2 := CalculateRetryWaitTimeMS(retryPolicy, 2)
	s.Assert().Equal(3000, waitTime2)

	// Third retry attempt should be 4.5 seconds using
	// first_retry_delay * backoff_factor^(retry_attempt_number-1).
	waitTime3 := CalculateRetryWaitTimeMS(retryPolicy, 3)
	s.Assert().Equal(4500, waitTime3)

	// Fourth retry attempt should be 6.75 seconds using
	// first_retry_delay * backoff_factor^(retry_attempt_number-1).
	waitTime4 := CalculateRetryWaitTimeMS(retryPolicy, 4)
	s.Assert().Equal(6750, waitTime4)

	// Fifth retry attempt should be 10.125 seconds using
	// first_retry_delay * backoff_factor^(retry_attempt_number-1).
	waitTime5 := CalculateRetryWaitTimeMS(retryPolicy, 5)
	s.Assert().Equal(10125, waitTime5)

	// Sixth retry attempt should be 15.1875 seconds using
	// first_retry_delay * backoff_factor^(retry_attempt_number-1).
	// However max_delay is set to 14
	// so the wait time should be capped at 14 seconds.
	waitTime6 := CalculateRetryWaitTimeMS(retryPolicy, 6)
	s.Assert().Equal(14000, waitTime6)
}

func (s *CalculateRetryWaitTimeMSTestSuite) Test_calculates_wait_time_in_ms_with_jitter() {
	retryPolicy := &RetryPolicy{
		FirstRetryDelay: 3,
		BackoffFactor:   2.0,
		MaxDelay:        80,
		Jitter:          false,
	}

	// First retry attempt would be 3 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// therefore between 0 and 3 seconds with jitter.
	waitTime1 := CalculateRetryWaitTimeMS(retryPolicy, 1)
	s.Assert().LessOrEqual(waitTime1, 3000)

	// Second retry attempt would be 6 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// therefore between 0 and 6 seconds with jitter.
	waitTime2 := CalculateRetryWaitTimeMS(retryPolicy, 2)
	s.Assert().LessOrEqual(waitTime2, 6000)

	// Third retry attempt would be 12 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// therefore between 0 and 12 seconds with jitter.
	waitTime3 := CalculateRetryWaitTimeMS(retryPolicy, 3)
	s.Assert().LessOrEqual(waitTime3, 12000)

	// Fourth retry attempt would be 24 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// therefore between 0 and 24 seconds with jitter.
	waitTime4 := CalculateRetryWaitTimeMS(retryPolicy, 4)
	s.Assert().LessOrEqual(waitTime4, 24000)

	// Fifth retry attempt would be 48 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// therefore between 0 and 48 seconds with jitter.
	waitTime5 := CalculateRetryWaitTimeMS(retryPolicy, 5)
	s.Assert().LessOrEqual(waitTime5, 48000)

	// Sixth retry attempt would be 96 seconds using
	// first_retry_delay * backoff_rate^(retry_attempt_number-1),
	// However max_delay is set to 80
	// so the wait time should be capped at 80 seconds.
	waitTime6 := CalculateRetryWaitTimeMS(retryPolicy, 6)
	s.Assert().Equal(80000, waitTime6)
}

func TestCalculateRetryWaitTimeMSTestSuite(t *testing.T) {
	suite.Run(t, new(CalculateRetryWaitTimeMSTestSuite))
}
