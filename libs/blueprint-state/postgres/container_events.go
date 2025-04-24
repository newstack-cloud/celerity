package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/core"
)

var (
	flushQueueWaitTime = 100 * time.Millisecond
	endSignalWaitTime  = 10 * time.Millisecond
)

type eventsContainerImpl struct {
	connPool *pgxpool.Pool
	logger   core.Logger
}

func (e *eventsContainerImpl) Get(
	ctx context.Context,
	ID string,
) (manage.Event, error) {
	var event manage.Event
	err := e.connPool.QueryRow(
		ctx,
		eventQuery(),
		&pgx.NamedArgs{
			"id": ID,
		},
	).Scan(&event)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, pgx.ErrNoRows) ||
			(errors.As(err, &pgErr) && isAltNotFoundPostgresErrorCode(pgErr.Code)) {
			return manage.Event{}, manage.EventNotFoundError(ID)
		}

		return manage.Event{}, err
	}

	if event.ID == "" {
		return manage.Event{}, manage.EventNotFoundError(ID)
	}

	return event, nil
}

func (e *eventsContainerImpl) Save(
	ctx context.Context,
	event *manage.Event,
) error {
	qInfo := prepareSaveEventQuery(event)
	_, err := e.connPool.Exec(
		ctx,
		qInfo.sql,
		qInfo.params,
	)
	if err != nil {
		return err
	}

	channel := eventsChannel(event.ChannelType, event.ChannelID)
	_, err = e.connPool.Exec(
		ctx,
		"SELECT pg_notify(@channel, @payload)",
		&pgx.NamedArgs{
			"channel": channel,
			"payload": event.ID,
		},
	)
	if err != nil {
		// No need to rollback if the notification fails,
		// listeners should first query the events table for existing events
		// for a given channel type and ID.
		return err
	}

	return nil
}

func (e *eventsContainerImpl) Stream(
	ctx context.Context,
	params *manage.EventStreamParams,
	streamTo chan manage.Event,
	errChan chan error,
) (chan struct{}, error) {
	streamLogger := e.logger.Named("eventStream").
		WithFields(
			core.StringLogField("channelType", params.ChannelType),
			core.StringLogField("channelId", params.ChannelID),
		)

	// In order to listen for notifications,
	// we need to acquire a connection from the pool as a listener
	// must be tied to a single session (connection).
	listenConn, err := e.connPool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	channel := eventsChannel(params.ChannelType, params.ChannelID)
	endChan := make(chan struct{})
	go e.streamEvents(
		ctx,
		params,
		channel,
		listenConn,
		&streamEventChannels{
			streamTo: streamTo,
			errChan:  errChan,
			endChan:  endChan,
		},
		streamLogger,
	)

	return endChan, nil
}

func (e *eventsContainerImpl) streamEvents(
	ctx context.Context,
	params *manage.EventStreamParams,
	channelName string,
	conn *pgxpool.Conn,
	channels *streamEventChannels,
	logger core.Logger,
) {
	defer unlistenAndRelease(ctx, conn, channelName, logger)

	// Listen before querying for existing events to ensure that
	// we do not miss any events that are sent during the initial query.
	_, err := conn.Exec(
		ctx,
		// LISTEN does not allow for parameters, so we have to use string interpolation
		// to build the query.
		// This is safe as long as the host application does not allow arbitrary
		// user input to be passed for the channel name.
		fmt.Sprintf("LISTEN %q", channelName),
	)
	if err != nil {
		channels.errChan <- err
		return
	}

	existingEvents, err := e.getChannelEvents(
		ctx,
		params,
		/* includeStartingEventID */ false,
	)
	if err != nil {
		return
	}

	for _, event := range existingEvents {
		select {
		case channels.streamTo <- event:
		case <-ctx.Done():
			return
		}
	}

	collectedIDs := []string{}
	// Initialise last flush as the epoch time to ensure that
	// the first flush is sent immediately.
	lastFlush := time.Unix(0, 0)

	for {
		// At the start of each iteration, check if the caller
		// has sent a signal to stop the stream.
		// Allow up to 10 milliseconds for the signal to be sent.
		select {
		case <-channels.endChan:
			return
		case <-time.After(endSignalWaitTime):
			// Continue to wait for notifications.
		}

		// Send any collected events if we have surpassed the wait time
		// for sending batches of events.
		// Batching is used to reduce the number of queries made to the database
		// when dealing with a large number of events in a short time.
		if len(collectedIDs) > 0 && time.Since(lastFlush) > flushQueueWaitTime {
			var returnEarly bool
			collectedIDs, returnEarly = e.sendEvents(
				ctx,
				collectedIDs,
				channels,
			)
			if returnEarly {
				return
			}

			lastFlush = time.Now()
		}

		notification, err := waitForNotification(
			ctx,
			conn,
		)
		if err != nil {
			// A timeout is expected when there are no notifications
			// to process, so we can continue and flush the queue.
			if !pgconn.Timeout(err) {
				channels.errChan <- err
				return
			}
		}

		if notification != nil &&
			notification.Payload != "" &&
			// Avoid duplicate events as we start listening before making the initial query
			// in order to avoid missing any events.
			// However, we may still receive notifications for events that
			// were returned in the initial query results.
			!hasEvent(existingEvents, notification.Payload) {
			collectedIDs = append(collectedIDs, notification.Payload)
			sortEventIDs(collectedIDs)
		}
	}
}

func (e *eventsContainerImpl) sendEvents(
	ctx context.Context,
	collectedIDs []string,
	channels *streamEventChannels,
) ([]string, bool) {
	collectedIDsCopy := make([]string, len(collectedIDs))
	copy(collectedIDsCopy, collectedIDs)

	events, err := e.getEventsByIDs(
		ctx,
		collectedIDs,
	)
	if err != nil {
		channels.errChan <- err
		return nil, true
	}

	for _, event := range events {
		select {
		case channels.streamTo <- event:
			collectedIDsCopy = removeElement(collectedIDsCopy, event.ID)
		case <-channels.endChan:
			return nil, true
		case <-ctx.Done():
			return nil, true
		}
	}

	return collectedIDsCopy, false
}

func waitForNotification(
	ctx context.Context,
	conn *pgxpool.Conn,
) (*pgconn.Notification, error) {
	// Keep wait time short to ensure that we can flush the queue
	// when there are no more events to process or there is a long wait time
	// between events.
	ctxShortTimeout, cancel := context.WithTimeout(
		ctx,
		flushQueueWaitTime,
	)
	defer cancel()

	return conn.Conn().WaitForNotification(ctxShortTimeout)
}

func (e *eventsContainerImpl) getChannelEvents(
	ctx context.Context,
	params *manage.EventStreamParams,
	includeStartingEventID bool,
) ([]manage.Event, error) {
	qInfo := prepareChannelEventsQuery(
		params,
		includeStartingEventID,
	)

	rows, err := e.connPool.Query(
		ctx,
		qInfo.sql,
		qInfo.params,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[manage.Event])
}

func (e *eventsContainerImpl) getEventsByIDs(
	ctx context.Context,
	eventIDs []string,
) ([]manage.Event, error) {
	query := eventsByIDsQuery()
	rows, err := e.connPool.Query(
		ctx,
		query,
		pgx.NamedArgs{
			"ids": eventIDs,
		},
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByName[manage.Event])
}

func (e *eventsContainerImpl) Cleanup(
	ctx context.Context,
	thresholdDate time.Time,
) error {
	query := cleanupEventsQuery()
	_, err := e.connPool.Exec(
		ctx,
		query,
		pgx.NamedArgs{
			"cleanupBefore": thresholdDate,
		},
	)
	return err
}

func prepareChannelEventsQuery(
	eventStreamParams *manage.EventStreamParams,
	includeStartingEventID bool,
) *queryInfo {
	sql := channelEventsQuery(
		eventStreamParams,
		includeStartingEventID,
	)

	params := pgx.NamedArgs{
		"channelType": eventStreamParams.ChannelType,
		"channelId":   eventStreamParams.ChannelID,
	}
	if eventStreamParams.StartingEventID != "" {
		params["afterEventId"] = eventStreamParams.StartingEventID
	}

	return &queryInfo{
		sql:    sql,
		params: &params,
	}
}

func prepareSaveEventQuery(event *manage.Event) *queryInfo {
	sql := saveEventQuery()

	params := buildEventArgs(event)

	return &queryInfo{
		sql:    sql,
		params: params,
	}
}

func buildEventArgs(event *manage.Event) *pgx.NamedArgs {
	return &pgx.NamedArgs{
		"id":          event.ID,
		"eventType":   event.Type,
		"channelType": event.ChannelType,
		"channelId":   event.ChannelID,
		"data":        event.Data,
		"timestamp":   toUnixTimestamp(int(event.Timestamp)),
	}
}

func unlistenAndRelease(
	ctx context.Context,
	conn *pgxpool.Conn,
	channel string,
	logger core.Logger,
) {
	_, err := conn.Exec(
		ctx,
		// UNLISTEN does not allow for parameters, so we have to use string interpolation
		// to build the query.
		// This is safe as long as the host application does not allow arbitrary
		// user input to be passed for the channel name.
		fmt.Sprintf("UNLISTEN %q", channel),
	)
	if err != nil {
		logger.Error(
			"Unlisten failed for event notifications",
			core.StringLogField("channel", channel),
			core.ErrorLogField("error", err),
		)
	}
	conn.Release()
}

func eventsChannel(
	channelType string,
	channelID string,
) string {
	return fmt.Sprintf(
		"events_%s_%s",
		channelType,
		channelID,
	)
}

func sortEventIDs(ids []string) {
	slices.SortStableFunc(ids, func(a, b string) int {
		// Event IDs must be UUIDs as per the schema for the events table.
		// Whether or not the ID generator has used UUIDv7 is at the discretion
		// of the host application.
		// However, if UUIDv7 is not used, this will not sort the IDs correctly.
		uuidA, _ := uuid.Parse(a)
		uuidB, _ := uuid.Parse(b)

		return int(uuidA.Time() - uuidB.Time())
	})
}

func removeElement(slice []string, element string) []string {
	for i, v := range slice {
		if v == element {
			return slices.Delete(slice, i, i+1)
		}
	}
	return slice
}

func hasEvent(events []manage.Event, eventID string) bool {
	return slices.ContainsFunc(events, func(event manage.Event) bool {
		return event.ID == eventID
	})
}

type streamEventChannels struct {
	streamTo chan manage.Event
	errChan  chan error
	endChan  chan struct{}
}
