package bridge

import (
	"testing"

	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/stretchr/testify/assert"
)

func Test_eventNameFromRecord_empty(t *testing.T) {
	r := streamtypes.Record{EventName: ""}
	assert.Equal(t, "UNKNOWN", eventNameFromRecord(r))
}

func Test_eventNameFromRecord_insert(t *testing.T) {
	r := streamtypes.Record{EventName: streamtypes.OperationTypeInsert}
	assert.Equal(t, "INSERT", eventNameFromRecord(r))
}

func Test_eventNameFromRecord_modify(t *testing.T) {
	r := streamtypes.Record{EventName: streamtypes.OperationTypeModify}
	assert.Equal(t, "MODIFY", eventNameFromRecord(r))
}

func Test_eventNameFromRecord_remove(t *testing.T) {
	r := streamtypes.Record{EventName: streamtypes.OperationTypeRemove}
	assert.Equal(t, "REMOVE", eventNameFromRecord(r))
}

func Test_eventNameFromRecord_unknown_type(t *testing.T) {
	r := streamtypes.Record{EventName: "SOME_FUTURE_OP"}
	assert.Equal(t, "SOME_FUTURE_OP", eventNameFromRecord(r))
}

func Test_endpointWithoutScheme_http(t *testing.T) {
	assert.Equal(t, "localhost:9000", endpointWithoutScheme("http://localhost:9000"))
}

func Test_endpointWithoutScheme_https(t *testing.T) {
	assert.Equal(t, "secure.example.com:443", endpointWithoutScheme("https://secure.example.com:443"))
}

func Test_endpointWithoutScheme_no_scheme(t *testing.T) {
	assert.Equal(t, "localhost:9000", endpointWithoutScheme("localhost:9000"))
}

func Test_endpointWithoutScheme_short_string(t *testing.T) {
	assert.Equal(t, "abc", endpointWithoutScheme("abc"))
}
