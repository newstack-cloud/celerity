package postgres

import (
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

func eventQuery() string {
	return `
	SELECT
		json_build_object(
			'id', e.id,
			'type', e.type,
			'channelType', e.channel_type,
			'channelId', e.channel_id,
			'data', e.data::text,
			'timestamp', EXTRACT(EPOCH FROM e.timestamp)::bigint
		) As event_json
	FROM events e
	WHERE id = @id
	`
}

func saveEventQuery() string {
	return `
		INSERT INTO events (
			id,
			"type",
			channel_type,
			channel_id,
			data,
			"timestamp"
		) VALUES (
			@id,
			@eventType,
			@channelType,
			@channelId,
			@data,
			@timestamp
		)
		ON CONFLICT (id) DO NOTHING
	`
}

func channelEventsQuery(
	params *manage.EventStreamParams,
	includeStartingEventID bool,
) string {
	query := `
		SELECT
			e.id,
			e.type,
			e.channel_type as channelType,
			e.channel_id as channelId,
			e.data::text as data,
			EXTRACT(EPOCH FROM e.timestamp)::bigint as timestamp
		FROM events e
		WHERE channel_type = @channelType
			AND channel_id = @channelId
	`

	if params.StartingEventID != "" {
		query += fmt.Sprintf(`
			AND e.id %s @afterEventId
		`, comparisonOperator(includeStartingEventID))
	}

	return query
}

func eventsByIDsQuery() string {
	return `
		SELECT
			e.id,
			e.type,
			e.channel_type as channelType,
			e.channel_id as channelId,
			e.data::text as data,
			EXTRACT(EPOCH FROM e.timestamp)::bigint as timestamp
		FROM events e
		WHERE id = ANY(@ids)
	`
}

func comparisonOperator(inclusive bool) string {
	if inclusive {
		return ">="
	}
	return ">"
}
