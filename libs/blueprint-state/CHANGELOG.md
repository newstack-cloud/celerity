# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2025-04-27

### Fixed

- Adds fix to the `memfile` state container implementation to make sure that recently queued events are streamed even if the last event for a channel is an end of stream marker.
- Adds fix to the `postgres` state container implementation to make sure that recently queued events are streamed even if the last event for a channel is an end of stream marker.

## [0.2.0] - 2025-04-26

### Changed

- **Breaking change** - Improves event streaming behaviour to include a new `End` field in the `manage.Event` data type. This is used to to indicate the end of a stream of events.
- Updates the `memfile` state container implementation to make use of the new `End` field to end a stream early if the last saved event was an end of stream marker.
- Updates the `memfile` state container implementation to only fetch recently queued events when a starting event ID is not provided.
- Updates the `postgres` state container implementation to make use of the new `End` field to end a stream early if the last saved event was an end of stream marker.
- Updates the `postgres` state container implementation to only fetch recently queued events when a starting event ID is not provided.
- Updates the database migrations for the postgres state container to include the `end` column in the `events` table migration.

_Breaking changes will occur in early 0.x releases of this library._

## [0.1.0] - 2025-04-26

### Added

- Adds the `postgres` state container implementation that implements the `state.Container` interface from the blueprint framework along with the `manage.Validation`, `manage.Changesets` and `manage.Events` interfaces that are designed to be used by host applications such as the deploy engine.
- Adds a set of database migrations for the postgres state container implementation to be used with database migration tools or integrated into an installer for a host application.
- Adds the `memfile` state container implementation that implements the `state.Container` interface from the blueprint framework along with the `manage.Validation`, `manage.Changesets` and `manage.Events` interfaces that are designed to be used by host applications such as the deploy engine. This implementation uses an in-memory store for retrievals and persists writes to files on disk.
